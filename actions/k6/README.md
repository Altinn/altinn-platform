# K6 Action

## Running locally
`docker build -t k6/toolsimage ./actions/k6/ && docker run -e INPUT_TEST_SCRIPT_FILEPATH="./services/k6/first_test.js" -e INPUT_NAMESPACE="platform" -v .:/github/workspace --workdir /github/workspace --rm k6/tool
simage`


https://docs.github.com/en/actions/sharing-automations/creating-actions/about-custom-actions#creating-a-readme-file-for-your-action



## A detailed description of what the action does

## Required input and output arguments

## Optional input and output arguments

## Secrets the action uses

## Environment variables the action uses

 ## An example of how to use your action in a workflow





===========

# Hello world docker action

This action prints "Hello World" or "Hello" + the name of a person to greet to the log.

## Inputs

## `who-to-greet`

**Required** The name of the person to greet. Default `"World"`.

## Outputs

## `time`

The time we greeted you.

## Example usage

uses: actions/hello-world-docker-action@v2
with:
  who-to-greet: 'Mona the Octocat'
