#!/bin/bash
set -e

# dockerd を vfs ストレージドライバでバックグラウンド起動
sudo nohup dockerd --host=unix:///var/run/docker.sock --storage-driver=vfs > /tmp/dockerd.log 2>&1 &

# dockerd の起動を待つ
until docker info > /dev/null 2>&1; do
  echo "Waiting for dockerd..."
  sleep 1
done
# docker.sock のパーミッションを変更して一般ユーザーでもアクセス可能に
sudo chmod 666 /var/run/docker.sock

echo "dockerd started."
