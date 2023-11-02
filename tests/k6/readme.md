## k6 test project for automated tests

# Getting started


Install pre-requisites
## Install k6

*We recommend running the tests through a docker container.*

From the command line:

> docker pull grafana/k6


Further information on [installing k6 for running in docker is available here.](https://k6.io/docs/get-started/installation/#docker)


Alternatively, it is possible to run the tests directly on your machine as well.

[General installation instructions are available here.](https://k6.io/docs/get-started/installation/)


## Running tests

All tests are defined in `src/tests` and in the top of each test file an example of the cmd to run the test is available.

The command should be run from the root of the k6 folder.

>$> cd /test/k6

Run test suite by specifying filename.

For example:

>$>  docker-compose run k6 run /src/tests/eformidling.js -e env=*** -e subscriptionKey=***

The comand consists of three sections

`docker-compose run` to run the test in a docker container

`k6 run {path to test file}` pointing to the test file you want to run e.g. `/src/tests/eformidling.js`


`-e env=*** -e subscriptionKey=***` all environment variables that should be included in the request.
