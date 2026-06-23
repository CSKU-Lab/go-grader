# [0.7.0](https://github.com/CSKU-Lab/go-grader/compare/v0.6.0...v0.7.0) (2026-06-23)


### Bug Fixes

* **CS-107:** drop messages instead of requeuing on unrecoverable handler errors ([e1e5d03](https://github.com/CSKU-Lab/go-grader/commit/e1e5d0385d4f01ad7c7c6b898520db8d961f511e))


### Features

* **CS-217:** enforce system maximum safe limits when limit fields are zero or negative ([3e7302b](https://github.com/CSKU-Lab/go-grader/commit/3e7302bef798d2c831ee97df5042d3bdf6c33347))

# [0.6.0](https://github.com/CSKU-Lab/go-grader/compare/v0.5.0...v0.6.0) (2026-06-22)


### Features

* **CS-211:** regenerate genproto with Segment message in File ([8897d1e](https://github.com/CSKU-Lab/go-grader/commit/8897d1e7390b8c98ff08a1b74c0016eb7cbfc3ec))

# [0.5.0](https://github.com/CSKU-Lab/go-grader/compare/v0.4.0...v0.5.0) (2026-06-21)


### Bug Fixes

* **CS-209:** fix broadcast queue not reaching all workers ([7e7e2ec](https://github.com/CSKU-Lab/go-grader/commit/7e7e2eca0f3c0541fb4a540136af9d3c2c5f093d))


### Features

* **CS-197:** fall back to payload CompareScriptID when task has no compare script ([544c74a](https://github.com/CSKU-Lab/go-grader/commit/544c74a1197cc5d9833a277cbf8c643238820406))

# [0.4.0](https://github.com/CSKU-Lab/go-grader/compare/v0.3.5...v0.4.0) (2026-06-20)


### Features

* add workflow to auto-update isolate-with-compilers base image tag ([061e95b](https://github.com/CSKU-Lab/go-grader/commit/061e95b1279d480df2c2d733aaf921cee2e57dea))

## [0.3.5](https://github.com/CSKU-Lab/go-grader/compare/v0.3.4...v0.3.5) (2026-06-20)


### Bug Fixes

* guard against divide by zero when totalTestCases is 0 ([4445b1e](https://github.com/CSKU-Lab/go-grader/commit/4445b1ed532752d21f0596ea8d23bd40c1da85a0))

## [0.3.4](https://github.com/CSKU-Lab/go-grader/compare/v0.3.3...v0.3.4) (2026-06-20)


### Bug Fixes

* cleanup isolate box before init to handle crashed previous runs ([0945903](https://github.com/CSKU-Lab/go-grader/commit/0945903acaf2eea9fe29263d199b7a5e08cd468b))

## [0.3.3](https://github.com/CSKU-Lab/go-grader/compare/v0.3.2...v0.3.3) (2026-05-30)


### Bug Fixes

* restart consumers on RabbitMQ connection drop ([7e7a231](https://github.com/CSKU-Lab/go-grader/commit/7e7a231d11a59aa9bfd0458839cb48ad268af13a))

## [0.3.2](https://github.com/CSKU-Lab/go-grader/compare/v0.3.1...v0.3.2) (2026-05-23)


### Bug Fixes

* handle RabbitMQ connection drops with auto-reconnect ([fa2aaec](https://github.com/CSKU-Lab/go-grader/commit/fa2aaec3124fdda74100599db760b0216d40a958))

## [0.3.1](https://github.com/CSKU-Lab/go-grader/compare/v0.3.0...v0.3.1) (2026-05-20)


### Bug Fixes

* worker logging and master limit concurrency ([179e18b](https://github.com/CSKU-Lab/go-grader/commit/179e18bcc4201f0c2836f13e2b7af23e1d7756c3))
