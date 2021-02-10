package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiiuae/gosshgit"

	"github.com/google/uuid"
	"github.com/gosimple/slug"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

type Drone struct {
	Trusted      bool
	DeviceID     string
	PublicSSHKey string
	IP           net.IP // TODO: should we use net.IPAddr?
}

type Mission struct {
	Slug             string
	Name             string
	Drones           []*Drone
	WifiSecret       string
	WifiSSID         string
	GitServer        gosshgit.Server
	GitSSHServerPort int
	GitShutdown      func()
}

type BacklogItem struct {
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Status  string      `json:"status"`
	Payload interface{} `json:"payload"`
}

var (
	missions map[string]*Mission       = make(map[string]*Mission)
	drones   map[string]string         = make(map[string]string)
	backlog  map[string][]*BacklogItem = make(map[string][]*BacklogItem)
)

func getMissionsHandler(w http.ResponseWriter, r *http.Request) {
	type mission struct {
		Slug    string `json:"slug"`
		Name    string `json:"name"`
		GitPort int    `json:"git_port"`
	}
	response := make([]mission, 0)

	for slug, f := range missions {
		response = append(response, mission{
			Slug:    slug,
			Name:    f.Name,
			GitPort: f.GitSSHServerPort,
		})
	}
	writeJSON(w, response)
}

func createMissionHandler(w http.ResponseWriter, r *http.Request) {
	var requestBody struct {
		Slug           string   `json:"slug"`
		Name           string   `json:"name"`
		AllowedSSHKeys []string `json:"allowed_ssh_keys"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestBody)
	defer r.Body.Close()
	if err != nil {
		log.Printf("Could not decode body: %v", err)
		http.Error(w, "Malformed request body", http.StatusBadRequest)
		return
	}

	log.Printf("Create mission: %s", requestBody.Slug)

	if len(requestBody.Slug) == 0 {
		log.Printf("Provided slug is empty")
		http.Error(w, "Empty mission slug", http.StatusBadRequest)
		return
	}
	slug := slug.Make(requestBody.Slug)
	if slug != requestBody.Slug {
		log.Printf("Slug generated '%s' -> '%s' did not match", requestBody.Slug, slug)
		http.Error(w, "Invalid mission slug", http.StatusBadRequest)
		return
	}

	f := missions[slug]
	if f != nil {
		log.Printf("Mission with slug '%s' already exists", slug)
		http.Error(w, "Mission slug already taken", http.StatusBadRequest)
		return
	}

	g := gosshgit.New(fmt.Sprintf("%s/repositories", slug))

	err = g.Initialize()
	if err != nil {
		log.Printf("Could not initialize git server: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	err = g.InitBareRepo("mission.git")
	if err != nil {
		log.Printf("Could not initialize repository: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	for _, allowedSSHKey := range requestBody.AllowedSSHKeys {
		g.Allow(allowedSSHKey)
	}

	gitListener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Printf("Could not start listening for tcp: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	gitPort := gitListener.Addr().(*net.TCPAddr).Port

	f = &Mission{
		Slug:             slug,
		Name:             requestBody.Name,
		WifiSecret:       uuid.New().String(),
		WifiSSID:         uuid.New().String(),
		GitServer:        g,
		GitSSHServerPort: gitPort,
	}

	err = f.createInitialConfig()
	if err != nil {
		log.Printf("Could not create initial config: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		g.Close()
		g.DeleteRepo("mission.git")
		return
	}

	go g.Serve(gitListener)

	missions[slug] = f
	backlog[slug] = make([]*BacklogItem, 0)

	var response struct {
		GitPort   int    `json:"git_port"`
		PublicKey string `json:"public_key"`
	}
	response.GitPort = gitPort
	response.PublicKey = strings.TrimSuffix(string(ssh.MarshalAuthorizedKey(g.PublicKey())), "\n")

	writeJSON(w, response)
}

func deleteMissionHandler(w http.ResponseWriter, r *http.Request) {
	c := r.Context()
	params := httprouter.ParamsFromContext(c)
	slug := params.ByName("slug")

	log.Printf("Delete mission: %s", slug)

	f, ok := missions[slug]
	if !ok {
		// no such mission
		return
	}

	shutdownCtx, cancelFunc := context.WithTimeout(c, 2*time.Second)
	defer cancelFunc()
	err := f.GitServer.Shutdown(shutdownCtx)
	if err != nil {
		log.Printf("Could not shutdown git server: %v", err)
		log.Printf("Forcing the server to close")
		err = f.GitServer.Close()
		if err != nil {
			log.Printf("Could not forcefully close the server: %v", err)
		}
	}
	f.GitServer.DeleteRepo("mission.git")

	delete(missions, slug)
}

func assignDroneToMissionHandler(w http.ResponseWriter, r *http.Request) {
	c := r.Context()
	params := httprouter.ParamsFromContext(c)
	slug := params.ByName("slug")

	var requestBody struct {
		DeviceID string `json:"device_id"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	defer r.Body.Close()
	if err != nil {
		log.Printf("Could not decode body: %v", err)
		http.Error(w, "Malformed request body", http.StatusBadRequest)
		return
	}

	log.Printf("Assign drone: %s -> %s", requestBody.DeviceID, slug)

	f, ok := missions[slug]
	if !ok {
		log.Printf("Unknown mission: %s", slug)
		http.Error(w, "Unknown mission", http.StatusBadRequest)
		return
	}

	if !isDroneActive(requestBody.DeviceID) {
		log.Printf("Drone not active: %s", requestBody.DeviceID)
		http.Error(w, "Drone not active", http.StatusBadRequest)
		return
	}

	if fs, ok := drones[requestBody.DeviceID]; ok {
		log.Printf("Drone '%s' already part of mission %s", requestBody.DeviceID, fs)
		http.Error(w, "Drone already assigned", http.StatusBadRequest)
		return
	}

	msg, err := json.Marshal(struct {
		Command string
		Payload interface{}
	}{
		Command: "initialize-trust",
		Payload: "",
	})
	if err != nil {
		log.Printf("Could not marshal initialize-trust command: %v\n", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	pubtok := mqttClient.Publish(fmt.Sprintf("/devices/%s/commands/control", requestBody.DeviceID), 1, false, msg)
	if !pubtok.WaitTimeout(time.Second * 2) {
		log.Printf("Publish timeout")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	err = pubtok.Error()
	if err != nil {
		log.Printf("Could not publish message to MQTT broker: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	f.Drones = append(f.Drones, &Drone{
		Trusted:  false,
		DeviceID: requestBody.DeviceID,
		IP:       net.IP{}, // will be populated when the drone gets trusted
	})
	drones[requestBody.DeviceID] = slug
}

func addTaskToMissionBacklogHandler(w http.ResponseWriter, r *http.Request) {
	c := r.Context()
	params := httprouter.ParamsFromContext(c)
	slug := params.ByName("slug")

	var requestBody struct {
		ID       string      `json:"id"`
		Type     string      `json:"type"`
		Priority int64       `json:"priority"`
		Payload  interface{} `json:"payload"`
	}

	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		log.Printf("Could not read body: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(body, &requestBody)
	if err != nil {
		log.Printf("Could not decode body: %v", err)
		http.Error(w, "Malformed request body", http.StatusBadRequest)
		return
	}

	log.Printf("Add task: %s -> %s", requestBody.Type, slug)

	f, ok := missions[slug]
	if !ok {
		log.Printf("Unknown mission: %s", slug)
		http.Error(w, "Unknown mission", http.StatusBadRequest)
		return
	}

	// add task to git
	err = f.publishGitMessage("task-created", string(body))
	if err != nil {
		log.Printf("Could not add task to backlog: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// send update to all drones in the mission
	msg, err := json.Marshal(struct {
		Command string
		Payload interface{}
	}{
		Command: "update-backlog",
		Payload: "",
	})
	if err != nil {
		log.Printf("Could not marshal initialize-trust command: %v\n", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	for _, drone := range f.Drones {
		pubtok := mqttClient.Publish(fmt.Sprintf("/devices/%s/commands/control", drone.DeviceID), 1, false, msg)
		if !pubtok.WaitTimeout(time.Second * 2) {
			log.Printf("Publish timeout for '%v'", drone.DeviceID)
			continue
		}
		err = pubtok.Error()
		if err != nil {
			log.Printf("Could not publish message to MQTT broker for '%v': %v", drone.DeviceID, err)
			continue
		}
	}

	blog := backlog[slug]
	backlog[slug] = append(blog, &BacklogItem{requestBody.ID, requestBody.Type, "in-progress", requestBody.Payload})
}

func getMissionBacklogHandler(w http.ResponseWriter, r *http.Request) {
	c := r.Context()
	params := httprouter.ParamsFromContext(c)
	slug := params.ByName("slug")
	response, ok := backlog[slug]
	if !ok {
		log.Printf("Mission with slug '%s' not found", slug)
		http.Error(w, "", http.StatusNotFound)
		return
	}
	writeJSON(w, response)
}

// handle trust message from drone
// drone has initialized its ssh keys and is ready to be joined
func handleTrustMessage(deviceID string, payload []byte) {
	log.Printf("Handle trust: %s", deviceID)
	var trust struct {
		PublicSSHKey string `json:"public_ssh_key"`
	}
	err := json.Unmarshal(payload, &trust)
	if err != nil {
		log.Printf("Could not unmarshal trust message: %v", err)
		return
	}

	missionSlug, ok := drones[deviceID]
	if !ok {
		log.Printf("Drone not part of any mission")
		return
	}
	f := missions[missionSlug]
	for _, d := range f.Drones {
		if d.DeviceID != deviceID {
			continue
		}

		if d.Trusted {
			log.Printf("Drone '%s' already trusted!", deviceID)
			return
		}

		d.Trusted = true
		d.PublicSSHKey = trust.PublicSSHKey
		d.IP = net.ParseIP("127.0.0.1")
		break
	}
	// we have a new trusted drone -> update config
	err = f.publishGitMessage("drone-added", fmt.Sprintf("{ \"name\": \"%s\" }", deviceID))
	if err != nil {
		log.Printf("Could not publish git message: %v", err)
		return
	}

	f.GitServer.Allow(trust.PublicSSHKey)

	joinMissionPayload, err := json.Marshal(struct {
		GitServerAddress string `json:"git_server_address"`
		GitServerKey     string `json:"git_server_key"`
	}{
		GitServerAddress: fmt.Sprintf("%s:%d", "mission-control-svc", f.GitSSHServerPort),
		GitServerKey:     strings.TrimSuffix(string(ssh.MarshalAuthorizedKey(f.GitServer.PublicKey())), "\n"),
	})
	if err != nil {
		log.Printf("Could not marshal join-mission payload: %v\n", err)
		return
	}

	// ask the drone to join the mission
	msg, err := json.Marshal(struct {
		Command string
		Payload string
	}{
		Command: "join-mission",
		Payload: string(joinMissionPayload),
	})

	if err != nil {
		log.Printf("Could not marshal join-mission command: %v\n", err)
		return
	}

	log.Printf("Sending join-mission command: %s", deviceID)

	pubtok := mqttClient.Publish(fmt.Sprintf("/devices/%s/commands/control", deviceID), 1, false, msg)
	if !pubtok.WaitTimeout(time.Second * 2) {
		log.Printf("Publish timeout")
		return
	}
	err = pubtok.Error()
	if err != nil {
		log.Printf("Could not publish message to MQTT broker: %v", err)
		return
	}
}

type ConfigDrone struct {
	Name             string
	GitServerAddress string
	GitServerKey     string
	GitClientKey     string
}
type Config struct {
	Wifi struct {
		SSID   string
		Secret string
	}
	Drones []ConfigDrone `yaml:",omitempty"`
}

func (f *Mission) createInitialConfig() error {
	config := Config{}
	config.Wifi.SSID = f.WifiSSID
	config.Wifi.Secret = f.WifiSecret
	b, err := yaml.Marshal(config)
	if err != nil {
		log.Printf("Could not marshal config")
		return err
	}

	tmpPath := filepath.Join("tmp", uuid.New().String())
	repoPath := filepath.Join(f.Slug, "repositories", "mission.git")

	out, err := exec.Command("git", "clone", repoPath, tmpPath).CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not clone local repo", out)
		return err
	}

	err = ioutil.WriteFile(filepath.Join(tmpPath, "config.yaml"), b, 0644)
	if err != nil {
		log.Printf("Could not write config.yaml")
		return err
	}

	cmd := exec.Command("git", "add", "config.yaml")
	cmd.Dir = tmpPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not add config.yaml to commit", out)
		return err
	}
	cmd = exec.Command("git", "-c", "user.email=\"commander@cloud\"", "-c", "user.name=\"Commander\"", "commit", "-m", "Initial commit")
	cmd.Dir = tmpPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not create a commit", out)
		return err
	}
	cmd = exec.Command("git", "branch", "-m", "main")
	cmd.Dir = tmpPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not switch to main branch", out)
		return err
	}

	cmd = exec.Command("git", "push", "origin", "main")
	cmd.Dir = tmpPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not push to origin", out)
		return err
	}

	out, err = exec.Command("rm", "-rf", tmpPath).CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not remove temporary folder", out)
		return err
	}

	return nil
}

/*
func (f *Mission) updateConfig() error {
	config := Config{}
	config.Wifi.SSID = f.WifiSSID
	config.Wifi.Secret = f.WifiSecret
	for _, d := range f.Drones {
		if !d.Trusted {
			continue
		}
		config.Drones = append(config.Drones, ConfigDrone{
			Name:             d.DeviceID,
			GitServerAddress: fmt.Sprintf("ssh://git@%s:2222/mission.git", d.IP),
			GitServerKey:     "TODO",
			GitClientKey:     d.PublicSSHKey,
		})
	}
	b, err := yaml.Marshal(config)
	if err != nil {
		log.Printf("Could not marshal config: %v", err)
		return err
	}
	_ = b

	tmpPath := filepath.Join("tmp", uuid.New().String())
	repoPath := filepath.Join(f.Slug, "repositories", "mission.git")

	out, err := exec.Command("git", "clone", repoPath, tmpPath).CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not clone local repo", out)
		return err
	}

	err = ioutil.WriteFile(filepath.Join(tmpPath, "config.yaml"), b, 0644)
	if err != nil {
		log.Printf("Could not write config.yaml")
		return err
	}

	cmd := exec.Command("git", "add", "config.yaml")
	cmd.Dir = tmpPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not add config.yaml to commit", out)
		return err
	}
	cmd = exec.Command("git", "-c", "user.email=\"commander@cloud\"", "-c", "user.name=\"Commander\"", "commit", "-m", "Update config")
	cmd.Dir = tmpPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not create a commit", out)
		return err
	}

	cmd = exec.Command("git", "push", "origin", "main")
	cmd.Dir = tmpPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not push to origin", out)
		return err
	}

	out, err = exec.Command("rm", "-rf", tmpPath).CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not remove temporary folder", out)
		return err
	}

	return nil
}
*/

type Task struct {
	Type     string
	Priority int64
	Payload  string
}

func (f *Mission) publishGitMessage(messageType string, payload string) error {
	// task := Task{
	// 	Type:     taskType,
	// 	Priority: priority,
	// 	Payload:  payload,
	// }

	// b, err := yaml.Marshal(task)
	// if err != nil {
	// 	log.Printf("Could not marshal task")
	// 	return err
	// }

	// taskfile := fmt.Sprintf("backlog/%s.yaml", uuid.New().String())

	tmpPath := filepath.Join("tmp", uuid.New().String())
	repoPath := filepath.Join(f.Slug, "repositories", "mission.git")

	out, err := exec.Command("git", "clone", repoPath, tmpPath).CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not clone local repo", out)
		return err
	}

	taskfile, err := appendMessage(tmpPath, messageType, payload)
	if err != nil {
		log.Printf("%s\n\nCould not append message", out)
		return err
	}

	// _ = os.Mkdir(filepath.Join(tmpPath, "backlog"), 0644)

	// err = ioutil.WriteFile(filepath.Join(tmpPath, taskfile), b, 0644)
	// if err != nil {
	// 	log.Printf("Could not write %s", taskfile)
	// 	return err
	// }

	cmd := exec.Command("git", "add", taskfile)
	cmd.Dir = tmpPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not add %s to commit", out, taskfile)
		return err
	}
	cmd = exec.Command("git", "-c", "user.email=\"commander@cloud\"", "-c", "user.name=\"Commander\"", "commit", "-m", "Update backlog")
	cmd.Dir = tmpPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not create a commit", out)
		return err
	}

	cmd = exec.Command("git", "push", "origin", "main")
	cmd.Dir = tmpPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not push to origin", out)
		return err
	}

	out, err = exec.Command("rm", "-rf", tmpPath).CombinedOutput()
	if err != nil {
		log.Printf("%s\n\nCould not remove temporary folder", out)
		return err
	}

	return nil
}

func appendMessage(repoRootPath string, messageType string, message string) (string, error) {
	timestamp := time.Now().UTC()
	id := uuid.New().String()
	path := filepath.Join(repoRootPath, "cloud")
	file := filepath.Join(path, "outbox.log")

	os.Mkdir(path, os.ModeDir|os.ModePerm)
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
		return "", err
	}
	defer f.Close()

	ts := timestamp.Format("2006-01-02 15:04:05.000")
	content := fmt.Sprintf("%s %s %s %s\n", ts, id, messageType, message)
	if _, err := f.WriteString(content); err != nil {
		log.Println(err)
		return "", err
	}

	return "cloud/outbox.log", nil
}
