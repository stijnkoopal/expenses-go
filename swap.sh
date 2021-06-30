#!/bin/sh

gcloud container clusters get-credentials expenses --zone europe-west4-b --project expenses-plompstratie
kubens default
#with cloudsql:
#telepresence \
#  --to-pod 5432 \
#  --from-pod 9000 \
#  --from-pod 8558 \
#  --namespace default \
#  --swap-deployment expenses:expenses

#without cloudsql:
telepresence \
  --from-pod 9000 \
  --from-pod 8558 \
  --namespace default \
  --swap-deployment expenses:expenses
