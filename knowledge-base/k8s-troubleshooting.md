# Kubernetes Troubleshooting Guide

## Pod Pending

If a Pod is in Pending state, use kubectl describe pod <pod-name> to check events. Common reasons are insufficient resources or unbound PVCs.
