# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
