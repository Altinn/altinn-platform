# Reusable JS libraries to be used with K6 based tests

Import the library
```
import * as altinnK6Lib from 'https://raw.githubusercontent.com/Altinn/altinn-platform/refs/heads/main/libs/k6/slack/build/index.js'
```
Use it.
```
altinnK6Lib.postSlackMessage(data)
```
