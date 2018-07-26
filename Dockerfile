FROM scratch

ADD bin/eve-marketwatch /

ENTRYPOINT ["/eve-marketwatch"]
