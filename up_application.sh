set -eoux

DATABASE_PASSWORD=$(kubectl get secret --namespace default my-bd-mysql -o jsonpath="{.data.mysql-root-password}" | base64 -d)
DEPLOYMENT_FILE="./application/deployment.yaml"
POD_METRICS_PORT=2112
LOCAL_METRICS_PORT=7000

# Kill the process that is listen on LOCAL_METRICS_PORT
kill $( lsof -i:$LOCAL_METRICS_PORT -t) || true

minikube image build -t my-golang-app:latest ./application

kubectl delete -f $DEPLOYMENT_FILE || true
kubectl apply  -f $DEPLOYMENT_FILE

kubectl wait --for=condition=Ready pod/go-client 

# Exposing the pod metrics port to our localhost
kubectl port-forward pod/go-client $LOCAL_METRICS_PORT:$POD_METRICS_PORT &
