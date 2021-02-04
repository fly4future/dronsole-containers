# mission-control container

## Building and running container

Build and tag container
```
docker build -t tii-mission-control .
```

Run container in docker
```
docker run --rm -it -p 8082:8082 tii-mission-control <mqtt-broker-address>
```

## Creating mission and assigning drones

Create mission called "Bravo mission" with id "bravo"
```
curl -d '{"slug":"bravo","name":"Bravo mission"}' localhost:8082/missions
```

Assign drone "drone-313" to mission "bravo"
```
curl -d '{"device_id":"drone-313"}' localhost:8082/missions/bravo/drones
```

## Listing missions

```
curl localhost:8082/missions
```
