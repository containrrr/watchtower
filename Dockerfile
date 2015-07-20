FROM centurylink/ca-certs
MAINTAINER CenturyLink Labs <ctl-labs-futuretech@centurylink.com>
LABEL "com.centurylinklabs.watchtower"="true"

COPY watchtower /

ENTRYPOINT ["/watchtower"]
