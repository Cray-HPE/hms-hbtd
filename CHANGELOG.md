# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.24.0] - 2025-06-04

### Updated

- Updated image and module dependencies
	- hms-base v2.3.0
	- hms-hmetcd v1.13.0
	- hms-msgbus v1.13.1
- Explicitly closed all request and response bodies using hms-base functions
- Fixed bug with jq use in runSnyk.sh
- Internal tracking ticket: CASMHMS-6400

## [1.23.0] - 2025-03-25

### Security

- Updated image and module dependencies for security updates
- Updated to Go v1.24
- Added image-pprof Makefile support
- Internal tracking ticket: CASMHMS-6414

## [1.22.0] - 2025-01-08

### Added

- Added support for ppprof builds

## [1.21.0] - 2024-12-04

### Changed

- Updated go to 1.23

## [1.20.0] - 2023-09-08

### Added

- Added support for the VirtualNode type

## [1.19.1] - 2023-01-11

### Changed

- internal refactoring of HBTD, no function change

## [1.19.0] - 2022-07-19

### Changed

- Updated CT tests to hms-test:3.2.0 image to pick up Helm test enhancements and CVE fixes.

## [1.18.0] - 2022-06-22

### Changed

- Updated CT tests to hms-test:3.1.0 image as part of Helm test coordination.

## [1.17.0] - 2022-05-23

### Changed

- convert HSM v1 to HSM v2.

## [1.16.0] - 2022-05-16

### Changed

- Builds now use github instead of Jenkins

## [1.15.0] - 2021-12-20

### Added

- Now uses latest version of hms-msgbus, which now uses Confluent kafka interface.

## [1.14.0] - 2021-10-29

### Added

- Added /heartbeat/{xname} API for security.

## [1.13.0] - 2021-10-27

### Added

- CASMHMS-5055 - Added HBTD CT test RPM.

## [1.12.5] - 2021-09-21

### Changed

- Changed cray-service version to ~6.0.0

## [1.12.4] - 2021-09-08

### Changed

- Changed docker image to run as the user nobody.

## [1.12.3] - 2021-08-10

### Changed

- Added GitHub configuration files and fixed snyk warning.

## [1.12.2] - 2021-07-26

### Changed

- Github migration phases 2 and 3.

## [1.12.1] - 2021-07-12

### Security

- CASMHMS-4933 - Updated base container images for security updates.

## [1.12.0] - 2021-06-18

### Changed

- Bump minor version for CSM 1.2 release branch

## [1.11.0] - 2021-06-18

### Changed

- Bump minor version for CSM 1.1 release branch

## [1.10.5] - 2021-05-11

### Changed

- Fixed bug in HB checker loop exposed by slow ETCD response.

## [1.10.4] - 2021-04-27

### Changed

- Now HB state changes are submitted to HSM in bulk rather than a node at a time.

## [1.10.3] - 2021-04-14

### Changed

- Updated Dockerfiles to pull base images from Artifactory instead of DTR.

## [1.10.2] - 2021-03-03

### Changed

- Changed HB warning limit to 35.

## [1.10.1] - 2021-03-03

### Changed

- Increased the pod resource limits.

## [1.10.0] - 2021-02-02

### Changed

- Updated License files to MIT License

## [1.9.1] - 2021-01-21

### Changed

- Added User-Agent headers to all outbound HTTP requests.

## [1.9.0] - 2021-01-14

### Changed

- Updated license file.


## [1.8.0] - 2020-11-05

- CASMHMS-4202 - HBTD now has APIs to allow HB status queries.

## [1.7.3] - 2020-10-20

- CASMHMS-4105 - Updated base Golang Alpine image to resolve libcrypto vulnerability.

## [1.7.2] - 2020-10-02

- Added env vars to the base service chart for ETCD compaction and other performance improvements.

## [1.7.1] - 2020-10-01

- CASMHMS-4065 - Update base image to alpine-3.12.

## [1.7.0] - 2020-09-15

These are changes to charts in support of:

moving to Helm v1/Loftsman v1
the newest 2.x cray-service base chart
upgraded to support Helm v3
modified containers/init containers, volume, and persistent volume claim value definitions to be objects instead of arrays
the newest 0.2.x cray-jobs base chart
upgraded to support Helm v3

## [1.6.1] - 2020-06-30

- CASMHMS-3629 - Updated HBTD CT smoke test with new API test cases.

## [1.6.0] - 2020-06-26

- Bumped the base chart to 1.11.1 for ETCD improvements. Updated istio pod annotation to exclude ETCD.

## [1.5.3] - 2020-06-12

- Bumped the base chart to 1.8 for ETCD improvements.

## [1.5.2] - 2020-06-08

- Now processes heartbeat telemetry in the background.

## [1.5.1] - 2020-05-29

- HBTD Helm chart now supports  online upgrade/downgrade.

## [1.5.0] - 2020-05-15

- HBTD now waits until ETCD is operational, eliminating the need for wait-for-etcd job.

## [1.4.2] - 2020-05-04

- CASMHMS-2962 - change to use tusted baseOS image in docker build.

## [1.4.1] - 2020-04-07

- Changed the default warning and error timeouts to 20 (warn) and 60 (error) seconds.

## [1.4.0] - 2020-04-03

- HBTD now runs multi-instance.  For now running 3 instances; will auto-scale later as part of an overall scaling effort.

## [1.3.1] - 2020-04-01

- Fixed non-determinism in the HB tracker unit test which was causing occasional build failures.

## [1.3.0] - 2020-03-31

- Now queues notifications bound for HSM if HSM is not responsive until it becomes responsive.

## [1.2.3] - 2020-03-26

- Bumped base chart version to use ETCD config change.

## [1.2.2] - 2020-03-23

- Now recovers correctly from "death" by sensing no copies of HBTD have been running, and handles stale HB records correctly.

## [1.2.1] - 2020-01-28

- Implemented the health logic and update the swagger file

## [1.2.0] - 2020-01-28

### Changed

- CASMHMS-2637 - added readiness, liveness, and health endpoints for the service.

## [1.1.1] - 2019-10-24

### Changed

- Now re-uses http transport and client objects for outbound HTTP requests.

## [1.1.0] - 2019-05-14

### Removed

- `hmi-nfd` from this repo.

### Fixed

- Jenkinsfile now points to correct Dockerfile.

## [1.0.0] - 2019-05-13

### Added

- This is the initial release. It contains everything that was in `hms-services` at the time with the major exception of being `go mod` based now.

### Changed

### Deprecated

### Removed

### Fixed

### Security
