#!/usr/bin/env bash
set -e

if [[ "$1" == "enable" && "$2" == "--now" && "$3" == "mariadb" ]]; then
  mkdir -p /run/mysqld
  chown mysql:mysql /run/mysqld 2>/dev/null || true
  mysqld_safe --skip-networking=0 --bind-address=127.0.0.1 >/tmp/mariadb.log 2>&1 &
  sleep 6
  exit 0
fi

if [[ "$1" == "enable" && "$2" == "--now" && "$3" == "nginx" ]]; then
  nginx >/tmp/nginx.log 2>&1 || true
  exit 0
fi

if [[ "$1" == "reload" && "$2" == "nginx" ]]; then
  nginx -s reload >/tmp/nginx-reload.log 2>&1 || true
  exit 0
fi

if [[ "$1" == "enable" && "$2" == "--now" && "$3" == "panelx-control-plane.service" ]]; then
  set -a
  source /etc/panelx/control-plane.env
  set +a
  nohup /opt/panelx/bin/panelx-control-plane >/tmp/panelx-control-plane.log 2>&1 &
  sleep 2
  exit 0
fi

if [[ "$1" == "enable" && "$2" == "--now" && "$3" == "panelx-node-agent.service" ]]; then
  set -a
  source /etc/panelx/node-agent.env
  set +a
  nohup /opt/panelx/bin/panelx-node-agent >/tmp/panelx-node-agent.log 2>&1 &
  sleep 2
  exit 0
fi

if [[ "$1" == "daemon-reload" ]]; then
  exit 0
fi

if [[ "$1" == "--no-pager" && "$2" == "--full" && "$3" == "status" ]]; then
  exit 0
fi

exit 0
