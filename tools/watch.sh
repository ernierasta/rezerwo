#!/bin/sh

# run all watchers
tools/sass --watch scss:css --no-source-map &
tools/air
