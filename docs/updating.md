## Updating Watchtower

If watchtower is monitoring the same Docker daemon under which the watchtower container itself is running (i.e. if you 
volume-mounted `/var/run/docker.sock` into the watchtower container) then it has the ability to update itself.  
If a new version of the `containrrr/watchtower` image is pushed to the Docker Hub, your watchtower will pull down the 
new image and restart itself automatically.
