# go-machine-service
The service will create, update, and destory Docker hosts using [Docker machine](https://github.com/docker/machine).

It is an [external event handler](https://github.com/rancherio/cattle/blob/master/docs/examples/handler-bash/simple_handler.sh) in Rancher that listens for events related to the life cycle of ```DockerMachine``` resources. In the context Rancher, ```DockerMachine``` is a subtype of ```PhysicalHost```. 

The following are the most interesting and important events that this service responds to:
* ```physicalhost.create``` - Calls ```machine create ...```.
* ```physicalhost.activate``` - Runs ```docker run rancher/agent ...``` on the host to bootstrap it into Rancher.
* ```physicalhost.delete|purge``` - Calls ```machine delete ...```
