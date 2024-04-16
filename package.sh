#!/bin/bash
packDir=gootify_v1
mkdir -p ./$packDir
go build
mv ./gootify ./$packDir
cp ./static/* ./$packDir
cp ./exampleEnv ./$packDir/.env
cp ./start.sh ./$packDir
cp ./README.md ./$packDir

tar -zcvf gootify.tar ./$packDir
rm -rf ./$packDir

