# Cray Heartbeat Tracking Service (hbtd)

The Shasta Heartbeat Tracking Service is a service which tracks the 
heartbeats emitted by various system components.  Generally, compute nodes
emit heartbeats to inform the HMS services that they are alive and healthy.
Other components can also emit heartbeats if they so choose.

The Shasta Heartbeat Tracking Service uses a RESTful interface to provide an
end point to which to send heartbeat packets.  

The Shasta Heartbeat Tracking Service will track heartbeats received and 
check them against the time of the previous heartbeat received for a given
component.   Changes in heartbeat behavior will be communicated to the
Hardware State manager:

* First time heartbeat received (HSM places component in READY state)
* Heartbeat missing -- warning situation.  Component may be dead (HSM places component in READY state with a WARNING flag)
* Heartbeat missing -- alert situation.  Component is dead (HSM places component in STANDBY state with an ALERT flag)

Normally _hbtd_ runs on the SMS cluster as one or more Docker container 
instances managed by Kubernetes.  It can also be run from a command shell 
for testing purposes.

## hbtd API

_hbtd_'s RESTful API is as follows:

```bash
/v1/heartbeat

    POST a heartbeat.
```

```bash
/v1/params

    GET or PATCH an hbdt operational parameter
```

See https://stash.us.cray.com/projects/HMS/repos/hms-hmi/browse/api/swagger.yaml for details on the _hbtd_ RESTful API payloads and return values.

## hbtd Command Line

```bash
Usage: hbtd [options]

  --help                  Help text.
  --debug=num             Debug level.  (Default: 0)
  --use_telemetry=yes|no  Inject notifications into message.
                          bus. (Default: yes)
  --telemetry_host=h:p:t  Hostname:port:topic of telemetry service
  --warntime=secs         Seconds before sending a warning of
                          node heartbeat failure.  
                          (Default: 10 seconds)
  --errtime=secs          Seconds before sending an error of
                          node heartbeat failure.  
                          (Default: 30 seconds)
  --interval=secs         Heartbeat check interval.
                          (Default: 5 seconds)
  --port=num              HTTPS port to listen on.  (Default: 28500)
  --kv_url=url            Key-Value service 'base' URL..  
                              (Default: https://localhost:2379)
  --sm_url=url            State Manager 'base' URL.  
                              (Default: http://localhost:27779/hsm/v1)
  --sm_retries=num        Number of State Manager access retries. (Default: 3)
  --sm_timeout=secs       State Manager access timeout. (Default: 10)
  --nosm                  Don't contact State Manager (for testing).
```

## Building And Executing hbtd

### Building hbtd

[Building _hbtd_ after the Repo split](https://connect.us.cray.com/confluence/display/CASMHMS/HMS+Repo+Split)

### Running hbtd Locally

Starting _hbtd_:

```bash
./hbtd --sm_url=https://localhost:27999/hsm/smd/v1 --port=28501 --use_telemetry=no --kv_url="mem:"
```

### Running hbtd In A Docker Container

From the root of this repo, build the docker container:

```bash
# docker build -t cray/cray-hbtd:test .
```

Then run (add `-d` to the arguments list of `docker run` to run in detached/background mode):

```bash
docker run -p 28500:28500 --name hbtd cray/hbtd:test
```

### hbtd CT Testing

This repository builds and publishes hms-hbtd-ct-test RPMs along with the service itself containing tests that verify hbtd on the
NCNs of live Shasta systems. The tests require the hms-ct-test-base RPM to also be installed on the NCNs in order to execute.
The version of the test RPM installed on the NCNs should always match the version of hbtd deployed on the system.

## Feature Map

| V1 Feature | V1+ Feature | XC Equivalent |
| --- | --- | --- |
| /v1/heartbeat | /v1/heartbeat | HB Via HW to Node Ctlr | 
| /v1/params | /v1/params | - | 
| - | Ability to query service health | - |
| - | Ability to dump service internals | - |


## Current Features

* Ability to receive heartbeats from components
* Ability to track component heartbeats
* Ability to notify Hardware State Manager if heartbeat status changes
* Ability to query service operating parameters
* Ability to modify service operating parameters

## Future Features And Updates

* Performance optimizations:
  * Ability to "forget" components which are going away/powering down
* Add API to query service health and connectivity
* Add API to dump service internal


