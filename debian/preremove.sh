#!/bin/sh

case "$1" in
    remove)
        systemctl disable texd || true
        systemctl stop texd    || true
    ;;
esac
