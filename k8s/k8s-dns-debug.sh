#!/bin/bash

NAMESPACE=${1:-analytics-platform}
SERVICE=${2:-kafka-0.kafka-headless}

set -e

echo "Checking CoreDNS pods in kube-system..."
kubectl -n kube-system get pods -l k8s-app=kube-dns

echo
echo "Checking kube-dns service..."
kubectl -n kube-system get svc kube-dns

echo
echo "Checking CoreDNS logs for errors..."
kubectl -n kube-system logs -l k8s-app=kube-dns --tail=20

echo
echo "Launching a busybox pod to test DNS in namespace: $NAMESPACE"
kubectl -n $NAMESPACE delete pod busybox-dns --ignore-not-found=true
kubectl -n $NAMESPACE run busybox-dns --rm -it --image=busybox --restart=Never -- sh -c "\
  echo 'Contents of /etc/resolv.conf:'; cat /etc/resolv.conf; \
  echo; \
  echo 'Testing DNS resolution for $SERVICE.$NAMESPACE.svc.cluster.local:'; \
  ping -c 3 $SERVICE.$NAMESPACE.svc.cluster.local || true; \
  echo; \
  echo 'Testing DNS resolution for kube-dns:'; \
  ping -c 3 kube-dns.kube-system.svc.cluster.local || true; \
  exit \
" 