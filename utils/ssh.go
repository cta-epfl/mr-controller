package utils

import (
	"log"
	"os"
)

func InitSshConfig() {

	if _, err := os.Stat("/home/app/.ssh"); os.IsNotExist(err) {
		err := os.Mkdir("/home/app/.ssh", 0700)
		if err != nil {
			log.Fatalf("Error creating .ssh folder: %s\n", err)
		}
	} else {
		err := os.Chmod("/home/app/.ssh", 0700)
		if err != nil && !os.IsPermission(err) {
			log.Fatalf("Error while checking permissions of .ssh folder: %s\n", err)
		}
	}

	fileConfig, err := os.Create("/home/app/.ssh/config")
	fileConfig.Write([]byte("IdentityFile /home/app/.ssh/id_ecdsa\n"))
	if err != nil && !os.IsPermission(err) {
		log.Fatalf("Error while creating ssh config file: %s\n", err)
	}

	fileKnownHosts, err := os.Create("/home/app/.ssh/known_hosts")
	fileKnownHosts.Write([]byte(os.Getenv("FLUX_KNOWN_HOSTS")))
	if err != nil && !os.IsPermission(err) {
		log.Fatalf("Error while creating ssh known_hosts file: %s\n", err)
	}

	fileEcdsa, err := os.Create("/home/app/.ssh/id_ecdsa")
	fileEcdsa.Write([]byte(os.Getenv("FLUX_IDENTITY")))
	fileEcdsa.Chmod(0600)
	if err != nil && !os.IsPermission(err) {
		log.Fatalf("Error while creating ssh id_ecdsa file: %s\n", err)
	}

	fileEcdsaPub, err := os.Create("/home/app/.ssh/id_ecdsa-pub")
	fileEcdsaPub.Write([]byte(os.Getenv("FLUX_IDENTITY_PUB")))
	fileEcdsaPub.Chmod(0644)
	if err != nil && !os.IsPermission(err) {
		log.Fatalf("Error while creating ssh id_ecdsa-pub: %s\n", err)
	}

	// cmd := exec.Command("cp", "/app/ssh-keys/*", "/home/app/.ssh")
	// err := cmd.Run()
	// if err != nil{
	// 	log.Printf("Error while ssh config folder: %s", err)
	// }

	// fileConfig, _ := os.Create("/home/app/.ssh/config")
	// fileConfig.Write([]byte("IdentityFile /home/app/.ssh/identity\n"))

	// os.Chmod("/home/app/.ssh/identity", 0600)
}
