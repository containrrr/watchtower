FROM centurylink/ca-certs
MAINTAINER CenturyLink Labs <innovationslab@ctl.io>
LABEL "com.centurylinklabs.watchtower"="true"

COPY watchtower /
ENTRYPOINT ["/watchtower"]
