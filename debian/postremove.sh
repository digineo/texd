#!/bin/sh

case "$1" in
    remove)
        systemctl daemon-reload
        userdel  texd || true
        groupdel texd 2>/dev/null || true
    ;;
esac
