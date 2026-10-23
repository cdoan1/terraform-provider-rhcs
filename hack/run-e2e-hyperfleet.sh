#!/bin/bash


export SEAPLANE_URL=$API_URL
export SEAPLANE_RELEASE_IMAGE=quay.io/openshift-release-dev/ocp-release:5.0.0-rc.5-multi
export CLUSTER_PROFILE=seaplane-basic

echo "$SEAPLANE_URL $SEAPLANE_RELEASE_IMAGE $CLUSTER_PROFILE"

TF_EXEC_STREAM_OUTPUT=true \
CLUSTER_PROFILE=seaplane-basic \
SEAPLANE_URL=$API_URL make e2e_test LabelFilter="day1-prepare"

