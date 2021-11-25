#!/bin/bash
echo Copiando archivos al resto de raspis...
go build main.go conections.go calc.go utils.go

scp -i pi_ca main pi@192.168.0.201:~/ &
scp -i pi_ca main pi@192.168.0.202:~/ &
scp -i pi_ca main pi@192.168.0.203:~/ &
wait
echo Copiando archivos a los contenedores...
sudo docker cp main pi:/ &
ssh -i pi_ca pi@192.168.0.201 sudo docker cp main pi:/ &
ssh -i pi_ca pi@192.168.0.202 sudo docker cp main pi:/ &
ssh -i pi_ca pi@192.168.0.203 sudo docker cp main pi:/ &
wait