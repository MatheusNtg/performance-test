set -eoux

DATABASE_PASSWORD=$(kubectl get secret --namespace default my-bd-mysql -o jsonpath="{.data.mysql-root-password}" | base64 -d)

# Builds a new image of our application
docker build -t my-golang-app:0.1.0 ./application

# Tags it to the image that goes to the registry
docker tag my-golang-app:0.1.0 localhost:5000/my-golang-app

# Pushes our image to the registry
docker push localhost:5000/my-golang-app

# Creates our pod to debug the application
kubectl run my-bd-mysql-client --rm --tty -i --restart='Never' --image=localhost:5000/my-golang-app --namespace default --env MYSQL_ROOT_PASSWORD=$DATABASE_PASSWORD --env MYSQL_SERVICE_ADDRS=$DATABASE_ADDRS --command -- bash