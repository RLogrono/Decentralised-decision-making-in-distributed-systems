#!/bin/bash
echo Actualizando "main" al servidor e inter...
cd ./../v2 ;go build ./../v2/main.go ./../v2/conections.go ./../v2/calc.go ./../v2/utils.go
scp -i ./../../.ssh/inter_ca ./../v2/main node@192.168.0.100:~/

docker cp ./../v2/main server:/opt/object_detection_video &
docker cp ./../v2/main inter:/opt/object_detection_video &
ssh -i ./../../.ssh/inter_ca node@192.168.0.100 docker cp main inter:/opt/object_detection_video/ &
wait

echo Actualizando archivos *.go a pi200...
scp -i ./../../.ssh/pi_ca ./../v2/main.go ./../v2/utils.go ./../v2/conections.go ./../v2/calc.go pi@192.168.0.200:~/v2/

echo Actualizando archivos de configuracion a todas las maquinas

scp -i ./../../.ssh/inter_ca ./../maquinas/inter/node.conf ./../maquinas/inter/time.conf ./../v2/data/decision-spec-rules.clp node@192.168.0.100:~/ &

scp -i ./../../.ssh/pi_ca ./../maquinas/pi200/node.conf ./../maquinas/pi200/time.conf ./../v2/data/decision-spec-rules.clp pi@192.168.0.200:~/ &
scp -i ./../../.ssh/pi_ca ./../maquinas/pi201/node.conf ./../maquinas/pi201/time.conf ./../v2/data/decision-spec-rules.clp pi@192.168.0.201:~/ &
scp -i ./../../.ssh/pi_ca ./../maquinas/pi202/node.conf ./../maquinas/pi202/time.conf ./../v2/data/decision-spec-rules.clp pi@192.168.0.202:~/ &
scp -i ./../../.ssh/pi_ca ./../maquinas/pi203/node.conf ./../maquinas/pi203/time.conf ./../v2/data/decision-spec-rules.clp pi@192.168.0.203:~/ &
wait

echo Actualizando archivos de configuracion a los contenedores
docker cp ./../maquinas/server/node.conf server:/opt/object_detection_video &
docker cp ./../maquinas/server/time.conf server:/opt/object_detection_video &
docker cp ./../v2/data/decision-spec-rules.clp server:/opt/object_detection_video &

docker cp ./../maquinas/inter/node.conf inter:/opt/object_detection_video &
docker cp ./../maquinas/inter/time.conf inter:/opt/object_detection_video &
docker cp ./../v2/data/decision-spec-rules.clp inter:/opt/object_detection_video &

ssh -i ./../../.ssh/inter_ca node@192.168.0.100 "docker cp ~/node.conf inter:/opt/object_detection_video;docker cp ~/time.conf inter:/opt/object_detection_video;docker cp decision-spec-rules.clp inter:/opt/object_detection_video/data" &
ssh -i ./../../.ssh/pi_ca pi@192.168.0.200 "sudo docker cp ~/node.conf pi:/; sudo docker cp ~/time.conf pi:/; sudo docker cp ~/decision-spec-rules.clp pi:/data/" &
ssh -i ./../../.ssh/pi_ca pi@192.168.0.201 "sudo docker cp ~/node.conf pi:/; sudo docker cp ~/time.conf pi:/; sudo docker cp ~/decision-spec-rules.clp pi:/data/" &
ssh -i ./../../.ssh/pi_ca pi@192.168.0.202 "sudo docker cp ~/node.conf pi:/; sudo docker cp ~/time.conf pi:/; sudo docker cp ~/decision-spec-rules.clp pi:/data/" &
ssh -i ./../../.ssh/pi_ca pi@192.168.0.203 "sudo docker cp ~/node.conf pi:/; sudo docker cp ~/time.conf pi:/; sudo docker cp ~/decision-spec-rules.clp pi:/data/" &
wait
