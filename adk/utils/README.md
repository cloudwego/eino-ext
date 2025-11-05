# Eino ADK utils

This repo provides various auxiliary tools for the Eino ADK.

## Installation
```
go get github.com/cloudwego/eino-ext/adk/utils
```

## Features
1. `utils/runner_wrapper`: provides NewRunner, which creates agent runner that enables global callbacks. It is very effective when you need to run callbacks at the agent runner layer.