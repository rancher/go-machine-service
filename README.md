# go-machine-service
The service will create, update, and destory Docker hosts using [Docker machine](https://github.com/docker/machine).

It is an [external event handler](https://github.com/rancherio/cattle/blob/master/docs/examples/handler-bash/simple_handler.sh) in Rancher that listens for events related to the life cycle of ```DockerMachine``` resources. In the context Rancher, ```DockerMachine``` is a subtype of ```PhysicalHost```. 

The following are the most interesting and important events that this service responds to:
* ```physicalhost.create``` - Calls ```machine create ...```.
* ```physicalhost.activate``` - Runs ```docker run rancher/agent ...``` on the host to bootstrap it into Rancher.
* ```physicalhost.delete|purge``` - Calls ```machine delete ...```

# License
Copyright (c) 2014-2015 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

