Watchtower will detect if there are links between any of the running containers and ensures that things are stopped/started in a way that won't break any of the links. If an update is detected for one of the dependencies in a group of linked containers, watchtower will stop and start all of the containers in the correct order so that the application comes back up correctly.

For example, imagine you were running a _mysql_ container and a _wordpress_ container which had been linked to the _mysql_ container. If watchtower were to detect that the _mysql_ container required an update, it would first shut down the linked _wordpress_ container followed by the _mysql_ container. When restarting the containers it would handle _mysql_ first and then _wordpress_ to ensure that the link continued to work.

If you want to override existing links, or if you are not using links, you can use special `com.centurylinklabs.watchtower.depends-on` label with dependent container names, separated by a comma.

When you have a depending container that is using `network_mode: service:container` then watchtower will treat that container as an implicit link.
