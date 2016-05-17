#!/bin/bash

# Deploy image to Docker Hub
docker login -e ${DOCKER_EMAIL} -u ${DOCKER_USER} -p ${DOCKER_PASS} && \
docker push skidder/streammarker-writer:latest
