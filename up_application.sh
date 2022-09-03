set -eoux

FIRST_INSTALLATION=false
DATABASE_NAME="my-bd"
DEPLOYMENT_FILE="./application/deployment.yaml"
DATABASE_PASSWORD="SuperSafePassword123@!"
POD_METRICS_PORT=2112
LOCAL_METRICS_PORT=7000
CONFIGMAP_NAME="test-config"
PROMETHEUS_DIR_PATH="$HOME/prometheus-2.38.0.linux-amd64"

echo "Checking if docker service is running"
if [[ $(sudo service docker status) == *"is not running" ]]; then
    echo "Docker service is not running, starting docker service"
    sudo service docker start
    FIRST_INSTALLATION=true
fi

echo "Checking if minikube is started"
if [[ $(minikube status) == *"Stopped" ]]; then
    echo "Minikube is not started, starting it"
    minikube start
fi

if [[ $FIRST_INSTALLATION == true ]]; then
    kubectl delete statefulset.apps/my-bd-mysql-primary
    kubectl delete statefulset.apps/my-bd-mysql-secondary
    kubectl delete pvc --all
fi

main() {
    local number_of_replicas=${1:-3}

    if [[ $(kubectl get pods | grep "$DATABASE_NAME" | wc -l) != $number_of_replicas ]]; then
        echo "Current number of replicas is different of the desired"
        echo "Fixing that"
        local timeout=$(($number_of_replicas/3))


        if [[ $(kubectl get pods | grep "$DATABASE_NAME" | wc -l) > 0 ]]; then
            echo "Upgrading current installation"
            helm upgrade $DATABASE_NAME bitnami/mysql \
                -f values.yaml \
                --set secondary.replicaCount=$number_of_replicas \
                --timeout $(($timeout*300))s \
                --wait
        fi

        if [[ $(kubectl get pods | grep "$DATABASE_NAME" | wc -l) == 0 ]]; then
            echo "Installing from the ground"
            helm delete $DATABASE_NAME 
            helm install $DATABASE_NAME \
                -f values.yaml \
                --set secondary.replicaCount=$number_of_replicas \
                --timeout $(($timeout*300))s \
                --wait \
                bitnami/mysql | tee instructions.txt
        fi
    fi

    # Kill the process that is listen on LOCAL_METRICS_PORT
    kill $( lsof -i:$LOCAL_METRICS_PORT -t) || true

    # Delete prometheus data folder
    rm -rf $PROMETHEUS_DIR_PATH/data || true

    # Up prometheus
    (cd $PROMETHEUS_DIR_PATH && ./prometheus &)
    minikube image build -t my-golang-app:latest ./application

    if [[ $(sudo service grafana-server status) == *"not running" ]]; then
        sudo service grafana-server start
    fi

    kubectl delete -f $DEPLOYMENT_FILE || true
    if [[ $(kubectl get cm | grep $CONFIGMAP_NAME | wc -l) == 0 ]]; then
        kubectl create configmap $CONFIGMAP_NAME --from-literal=database.replicas=$number_of_replicas
    else
        kubectl delete cm $CONFIGMAP_NAME
        kubectl create configmap $CONFIGMAP_NAME --from-literal=database.replicas=$number_of_replicas
    fi
    kubectl apply  -f $DEPLOYMENT_FILE

    kubectl wait --for=condition=Ready pod/go-client 

    # Exposing the pod metrics port to our localhost
    kubectl port-forward pod/go-client $LOCAL_METRICS_PORT:$POD_METRICS_PORT &

    kubectl logs go-client -f
}

main $@
