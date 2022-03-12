#!/bin/sh

groupadd --system texd || true
useradd --system -d /nonexistent -s /usr/sbin/nologin -g texd texd || true

systemctl daemon-reload
systemctl enable texd
systemctl restart texd
