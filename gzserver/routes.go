package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"syscall"
	"time"

	"github.com/julienschmidt/httprouter"
)

func registerRoutes(router *httprouter.Router) {
	router.HandlerFunc(http.MethodPost, "/simulation/start", startSimulationHandler)
	router.HandlerFunc(http.MethodPost, "/simulation/stop", stopSimulationHandler)

	router.HandlerFunc(http.MethodGet, "/simulation/drones", listDronesHandler)
	router.HandlerFunc(http.MethodPost, "/simulation/drones", createDroneHandler)
	router.HandlerFunc(http.MethodDelete, "/simulation/drones/:id", deleteDroneHandler)
}

type Drone struct {
	Location string
}

var (
	gzserverCmd *exec.Cmd
	drones      map[string]*Drone = make(map[string]*Drone)
)

func startSimulationHandler(w http.ResponseWriter, r *http.Request) {
	if gzserverCmd != nil {
		log.Printf("Simulation already running")
		http.Error(w, "Simulation already running", http.StatusBadRequest)
		return
	}

	var requestBody struct {
		WorldFile string `json:"world_file"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		log.Printf("Could not decode body: %v", err)
		http.Error(w, "Malformatted body", http.StatusBadRequest)
		return
	}
	log.Printf("Starting simulation")

	worldFile := requestBody.WorldFile
	if len(worldFile) == 0 {
		worldFile = "empty.world"
	}

	worldFile = fmt.Sprintf("/data/worlds/%s", worldFile)

	gzserverCmd, err = startCommandWithLogging("gzserver: ", "bash", "-c", fmt.Sprintf("/gzserver-api/scripts/launch-gzserver.sh %s", worldFile))
	if err != nil {
		log.Fatal(err)
	}
}
func stopSimulationHandler(w http.ResponseWriter, r *http.Request) {
	if gzserverCmd == nil {
		log.Printf("Simulation not running")
		http.Error(w, "Simulation not running", http.StatusBadRequest)
		return
	}
	log.Printf("Stopping simulation")
	syscall.Kill(-gzserverCmd.Process.Pid, syscall.SIGKILL)
	gzserverCmd = nil
}

func listDronesHandler(w http.ResponseWriter, r *http.Request) {
	if gzserverCmd == nil {
		log.Printf("Simulation not running")
		http.Error(w, "Simulation not running", http.StatusBadRequest)
		return
	}

	type drone struct {
		DeviceID      string `json:"device_id"`
		DroneLocation string `json:"drone_location"`
	}

	droneList := make([]drone, 0)
	for id, d := range drones {
		droneList = append(droneList, drone{
			DeviceID:      id,
			DroneLocation: d.Location,
		})
	}

	writeJSON(w, droneList)
}

func createDroneHandler(w http.ResponseWriter, r *http.Request) {
	if gzserverCmd == nil {
		log.Printf("Simulation not running")
		http.Error(w, "Simulation not running", http.StatusBadRequest)
		return
	}
	var requestBody struct {
		DroneLocation  string  `json:"drone_location"`
		DeviceID       string  `json:"device_id"`
		MAVLinkAddress string  `json:"mavlink_address"`
		MAVLinkUDPPort int32   `json:"mavlink_udp_port"`
		MAVLinkTCPPort int32   `json:"mavlink_tcp_port"`
		VideoUDPPort   int32   `json:"video_udp_port"`
		PosX           float64 `json:"pos_x"`
		PosY           float64 `json:"pos_y"`
		PosZ           float64 `json:"pos_z"`
		Pitch          float64 `json:"pitch"`
		Yaw            float64 `json:"yaw"`
		Roll           float64 `json:"roll"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		log.Printf("Could not decode body: %v", err)
		http.Error(w, "Malformatted body", http.StatusBadRequest)
		return
	}

	_, ok := drones[requestBody.DeviceID]
	if ok {
		log.Printf("Request to add drone with device id already in use")
		http.Error(w, "DeviceID already in use", http.StatusBadRequest)
		return
	}

	ips, err := net.LookupIP(requestBody.MAVLinkAddress)
	if err != nil {
		log.Printf("Could not lookup mavlink IP '%s': %v", requestBody.MAVLinkAddress, err)
		http.Error(w, "Cloud not lookup mavlink IP", http.StatusInternalServerError)
		return
	}
	if len(ips) == 0 {
		log.Printf("Could not lookup mavlink IP '%s': %v", requestBody.MAVLinkAddress, err)
		http.Error(w, "Cloud not lookup mavlink IP", http.StatusInternalServerError)
		return
	}
	command := fmt.Sprintf("/gzserver-api/scripts/spawn-drone.sh %s %d %d %d %s %f %f %f %f %f %f",
		ips[0].String(),
		requestBody.MAVLinkUDPPort,
		requestBody.MAVLinkTCPPort,
		requestBody.VideoUDPPort,
		requestBody.DeviceID,
		requestBody.PosX,
		requestBody.PosY,
		requestBody.PosZ,
		requestBody.Yaw,
		requestBody.Pitch,
		requestBody.Roll)

	// add drone model and connect it to the mavlink
	droneSpawnCmd, err := startCommandWithLogging(fmt.Sprintf("drone (%s): ", requestBody.DeviceID), "bash", "-c", command)
	if err != nil {
		log.Printf("Could not spawn drone model: %v", err)
		http.Error(w, "Could not spawn drone model", http.StatusInternalServerError)
		return
	}

	done := make(chan error)
	go func() { done <- droneSpawnCmd.Wait() }()
	select {
	case <-time.After(20 * time.Second):
		log.Printf("Spawn timeout after 20 seconds")
		droneSpawnCmd.Process.Kill()
	case err := <-done:
		if err != nil {
			log.Printf("Spawn failed: %v", err)
			http.Error(w, "Spawn failed", http.StatusInternalServerError)
		}
	}

	drones[requestBody.DeviceID] = &Drone{
		Location: requestBody.DroneLocation,
	}
}
func deleteDroneHandler(w http.ResponseWriter, r *http.Request) {
	if gzserverCmd == nil {
		log.Printf("Simulation not running")
		http.Error(w, "Simulation not running", http.StatusBadRequest)
		return
	}
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		log.Printf("Could not marshal data to json: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
