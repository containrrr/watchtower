Watchtower is an application that will monitor your running Docker containers and watch for changes to the images that those containers were originally started from. If watchtower detects that an image has changed, it will automatically restart the container using the new image.

With watchtower you can update the running version of your containerized app simply by pushing a new image to the Docker Hub or your own image registry. Watchtower will pull down your new image, gracefully shut down your existing container and restart it with the same options that were used when it was deployed initially.

For example, let's say you were running watchtower along with an instance of _centurylink/wetty-cli_ image:

```text
$ docker ps
CONTAINER ID   IMAGE                   STATUS          PORTS                    NAMES
967848166a45   centurylink/wetty-cli   Up 10 minutes   0.0.0.0:8080->3000/tcp   wetty
6cc4d2a9d1a5   containrrr/watchtower   Up 15 minutes                            watchtower
```

Every day watchtower will pull the latest _centurylink/wetty-cli_ image and compare it to the one that was used to run the "wetty" container. If it sees that the image has changed it will stop/remove the "wetty" container and then restart it using the new image and the same `docker run` options that were used to start the container initially (in this case, that would include the `-p 8080:3000` port mapping).

