#!/usr/bin/env bash

# Simulates a container that will always be updated, checking whether it shuts down it's dependencies correctly.
# Note that this test does not verify the results in any way

set -e
SCRIPT_ROOT=$(dirname "$(readlink -m "$(type -p "$0")")")
source "$SCRIPT_ROOT/docker-util.sh"

DepArgs=""
if [ -z "$1" ] || [ "$1" == "depends-on" ]; then
  DepArgs="--label com.centurylinklabs.watchtower.depends-on=parent"
elif [ "$1" == "linked" ]; then
  DepArgs="--link parent"
else
  DepArgs=$1
fi

WatchArgs="${*:2}"
if [ -z "$WatchArgs" ]; then
  WatchArgs="--debug"
fi

try-remove-container parent
try-remove-container depending

REPO=$(registry-host)

create-dummy-image deptest/parent
create-dummy-image deptest/depending

echo ""

echo -en "Starting \e[94mparent\e[0m container... "
CmdParent="docker run -d -p 9090 --name parent $REPO/deptest/parent"
$CmdParent
PARENT_REV_BEFORE=$(query-rev parent)
PARENT_START_BEFORE=$(container-started parent)
echo -e "Rev: \e[92m$PARENT_REV_BEFORE\e[0m"
echo -e "Started: \e[96m$PARENT_START_BEFORE\e[0m"
echo -e "Command: \e[37m$CmdParent\e[0m"

echo ""

echo -en "Starting \e[94mdepending\e[0m container... "
CmdDepend="docker run -d -p 9090 --name depending $DepArgs $REPO/deptest/depending"
$CmdDepend
DEPEND_REV_BEFORE=$(query-rev depending)
DEPEND_START_BEFORE=$(container-started depending)
echo -e "Rev: \e[92m$DEPEND_REV_BEFORE\e[0m"
echo -e "Started: \e[96m$DEPEND_START_BEFORE\e[0m"
echo -e "Command: \e[37m$CmdDepend\e[0m"

echo -e ""

create-dummy-image deptest/parent

echo -e "\nRunning watchtower..."

if [ -z "$WATCHTOWER_TAG" ]; then
  ## Windows support:
  #export DOCKER_HOST=tcp://localhost:2375
  #export CLICOLOR=1
  go run . --run-once $WatchArgs
else
  docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock containrrr/watchtower:"$WATCHTOWER_TAG" --run-once $WatchArgs
fi

echo -e "\nSession results:"

PARENT_REV_AFTER=$(query-rev parent)
PARENT_START_AFTER=$(container-started parent)
echo -en "  Parent image: \e[95m$PARENT_REV_BEFORE\e[0m => \e[94m$PARENT_REV_AFTER\e[0m "
if [ "$PARENT_REV_AFTER" == "$PARENT_REV_BEFORE" ]; then
  echo -e "(\e[91mSame\e[0m)"
else
  echo -e "(\e[92mUpdated\e[0m)"
fi
echo -en "  Parent container: \e[95m$PARENT_START_BEFORE\e[0m => \e[94m$PARENT_START_AFTER\e[0m "
if [ "$PARENT_START_AFTER" == "$PARENT_START_BEFORE" ]; then
  echo -e "(\e[91mSame\e[0m)"
else
  echo -e "(\e[92mRestarted\e[0m)"
fi

echo ""

DEPEND_REV_AFTER=$(query-rev depending)
DEPEND_START_AFTER=$(container-started depending)
echo -en "  Depend image: \e[95m$DEPEND_REV_BEFORE\e[0m => \e[94m$DEPEND_REV_AFTER\e[0m "
if [ "$DEPEND_REV_BEFORE" == "$DEPEND_REV_AFTER" ]; then
  echo -e "(\e[92mSame\e[0m)"
else
  echo -e "(\e[91mUpdated\e[0m)"
fi
echo -en "  Depend container: \e[95m$DEPEND_START_BEFORE\e[0m => \e[94m$DEPEND_START_AFTER\e[0m "
if [ "$DEPEND_START_BEFORE" == "$DEPEND_START_AFTER" ]; then
  echo -e "(\e[91mSame\e[0m)"
else
  echo -e "(\e[92mRestarted\e[0m)"
fi

echo ""