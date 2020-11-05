package main

import (
	"flag"
	"fmt"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"os"
	"strings"
)

func main() {
	var sourceDeploymentFileName = flag.String("deployment", "deployment.yaml", "source deployment filename")
	var configmapFileName = flag.String("configmap", "configmap.yaml", "source configMap filename")
	var destinationDeploymentFilename = flag.String("destination", "deployment.yaml", "destination deployment filename")
	flag.Parse()

	deployment, _ := parseDeployment(*sourceDeploymentFileName)
	configMaps, _ := parseConfigMap(*configmapFileName)

	replacedCount := 0

	for i := 0; i < len(deployment.Spec.Template.Spec.Containers); i++ {
		container := deployment.Spec.Template.Spec.Containers[i]
		for j, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
				value, err := getValueFromConfigMap(configMaps, env.ValueFrom.ConfigMapKeyRef.Name, env.ValueFrom.ConfigMapKeyRef.Key)
				if err != nil {
					log.Warnln(env.ValueFrom, err)
					continue
				}
				newEnvVar := newEnvVar(env.Name, value)
				container.Env = replace(container.Env, j, newEnvVar)
				replacedCount++
			}
		}

		deployment.Spec.Template.Spec.Containers[i] = container
	}

	marshal, _ := yaml.Marshal(deployment)
	err := ioutil.WriteFile(*destinationDeploymentFilename, marshal, os.ModePerm)
	if err != nil {
		log.Error(err)
	}

	log.Infoln("Replaced:", replacedCount)
}

func replace(envVars []core.EnvVar, index int, newEnv core.EnvVar) (result []core.EnvVar) {
	result = append(envVars[:index], newEnv)
	return append(result, envVars[index+1:]...)
}

func newEnvVar(name string, value string) core.EnvVar {
	return core.EnvVar{
		Name:  name,
		Value: value,
	}
}

func getValueFromConfigMap(conf []core.ConfigMap, confName string, fieldRef string) (string, error) {
	for _, configMap := range conf {
		if configMap.Name == confName {
			for key, value := range configMap.Data {
				if key == fieldRef {
					return value, nil
				}
			}
		}
	}
	return "", fmt.Errorf("not found")
}

func parseDeployment(fileName string) (*v1.Deployment, error) {
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	var deployment v1.Deployment
	err = yaml.Unmarshal(bytes, &deployment)
	if err != nil {
		return nil, err
	}

	return &deployment, nil
}

func parseConfigMap(fileName string) (result []core.ConfigMap, err error) {
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	for _, doc := range strings.Split(string(bytes), "---") {
		var configMap core.ConfigMap

		err = yaml.Unmarshal([]byte(doc), &configMap)
		if err != nil {
			log.Warn(err)
			continue
		}
		if configMap.Data != nil && configMap.Kind == "ConfigMap" {
			result = append(result, configMap)
		}
	}

	return result, nil
}
