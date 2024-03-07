image_name:=xdisco:usercase01

init-minikube:
	minikube start --insecure-registry "10.0.0.0/24,192.168.39.0/24" 
	minikube addons enable registry


init-rbac:
	minikube kubectl -- apply -f ./deployment-rbac.yml


image:
	DOCKER_BUILDKIT=1 docker build ./../../ -f ./Dockerfile  -t $(image_name) --cache-from $(image_name)


image-upload2:
	minikube image load $(image_name)


image-upload:
	docker tag $(image_name) $(shell minikube ip):5000/$(image_name)
	docker push $(shell minikube ip):5000/$(image_name)
 

deploy:
	minikube kubectl -- apply -f ./deployment.yml


undeploy:
	minikube kubectl -- delete -f ./deployment.yml


test: undeploy-minikube deploy-minikube


start-with-etcd:
	tmux new-session -d -s eg
	tmux split-window -t "eg:0"
	tmux split-window -t "eg:0.0" -h
	tmux split-window -t "eg:0.2" -h
	tmux send-keys -t "eg:0.0" "go run . server --broker etcd --port 0" Enter
	tmux send-keys -t "eg:0.1" "go run . server --broker etcd --port 0" Enter
	tmux send-keys -t "eg:0.2" "go run . server --broker etcd --port 0" Enter
	tmux send-keys -t "eg:0.3" "sleep 1" Enter "go run . client --broker etcd --username 9527" Enter
	tmux attach -t eg
	tmux kill-session -t eg

