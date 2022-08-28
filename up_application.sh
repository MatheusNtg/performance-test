set -eoux

echo "Checking if docker service is running"
if [[ $(sudo service docker status) == *"is not running" ]]; then
    echo "Docker service is not running, starting docker service"
    sudo service docker start
fi

echo "Checking if minikube is started"
if [[ $(minikube status) == *"Stopped" ]]; then
    echo "Minikube is not started, starting it"
    minikube start
fi

DATABASE_NAME="my-bd"
DEPLOYMENT_FILE="./application/deployment.yaml"
DATABASE_PASSWORD="SuperSafePassword123@!"
POD_METRICS_PORT=2112
LOCAL_METRICS_PORT=7000

main() {
    local number_of_replicas=${1:-3}

    if [[ $(kubectl get pods | grep "$DATABASE_NAME" | wc -l) != $number_of_replicas ]]; then
        echo "Current number of replicas is different of the desired"
        echo "Fixing that"
        local timeout=$(($number_of_replicas/3))
        helm delete $DATABASE_NAME || true
        helm install $DATABASE_NAME \
            -f values.yaml \
            --set secondary.replicaCount=$number_of_replicas \
            --timeout $(($timeout*300))s \
            --wait \
            bitnami/mysql
    fi

    # Kill the process that is listen on LOCAL_METRICS_PORT
    kill $( lsof -i:$LOCAL_METRICS_PORT -t) || true

    minikube image build -t my-golang-app:latest ./application

    kubectl delete -f $DEPLOYMENT_FILE || true
    kubectl apply  -f $DEPLOYMENT_FILE

    kubectl wait --for=condition=Ready pod/go-client 

    # Exposing the pod metrics port to our localhost
    kubectl port-forward pod/go-client $LOCAL_METRICS_PORT:$POD_METRICS_PORT &
}

main $@
