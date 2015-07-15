FROM centurylink/ca-certs
MAINTAINER CenturyLink Labs <ctl-labs-futuretech@centurylink.com>

COPY watchtower /

ENTRYPOINT ["/watchtower"]
