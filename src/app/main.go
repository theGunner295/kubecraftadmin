package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sandertv/mcwss/mctype"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/sandertv/mcwss/protocol/command"
	"github.com/sandertv/mcwss/protocol/event"

	"github.com/sandertv/mcwss"
)

var initpos mctype.Position
var initialized bool = false
var playerUniqueIdsMap = make(map[string][]string)
var selectednamespaces []string

var agent mcwss.Agent
var namespacesp []mctype.Position
var playerKubeMap = make(map[string][]string)
var playerEntitiesMap = make(map[string][]string)

// ENV paramaters
var passedNamespaces = os.Getenv("namespaces")
var accessWithinCluster = os.Getenv("accessWithinCluster")

//var accessWithinCluster = "yes"

func main() {
	if accessWithinCluster == "" {
		accessWithinCluster = "false"
	}

	initialized = false
	rand.Seed(86)

	//clientset, _ := GetClient(accessWithinCluster)
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		//kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "")
		kubeconfig = flag.String("kubeconfig", filepath.Join("/.kube", "config"), "")
		fmt.Print(home)
		fmt.Print(*kubeconfig)
	} else {
		kubeconfig = flag.String("kubeconfig", "", "/.kube/config")
		fmt.Print("Home is ''")
		fmt.Print(*kubeconfig)
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	/* 	for {
		namespacesno, _ := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d namespaces in the cluster\n", len(namespacesno.Items))
	} */

	// Create a new server using the default configuration. To use specific configuration, pass a *wss.Config{} in here.
	var c = mcwss.Config{HandlerPattern: "/ws", Address: "0.0.0.0:8000"}
	server := mcwss.NewServer(&c)

	fmt.Println("Listening on port 8000")

	// On first connection
	server.OnConnection(func(player *mcwss.Player) {
		uniqueIDs := make([]string, 0)
		playerUniqueIdsMap[player.Name()] = uniqueIDs

		//MOTD(player)
		MOTD(player)
		Actionbar(player, "Connected to k8s cluster")

		fmt.Println("Player ", player.Name(), " has entered!")
		player.Exec("time set noon", nil)
		player.Exec("weather clear", nil)
		player.Exec("alwaysday", nil)

		// Provide player with 'equipment'
		player.Exec("give @s diamond_sword", nil)
		player.Exec("give @s tnt 25", nil)
		player.Exec("give @s flint_and_steel", nil)

		playerName := player.Name()
		playerTravelMap := make(map[string]bool)
		playerTravelMap[playerName] = false

		playerInitMap := make(map[string]bool)
		playerInitMap[playerName] = false

		player.OnTravelled(func(event *event.PlayerTravelled) {
			player.Exec("testforblock ~ ~-1 ~ beacon", func(response *command.LocalPlayerName) {
				if response.StatusCode == 0 {
					GetPlayerPosition(player)
					SetNamespacesPositionByPos(initpos)
					if !playerInitMap[playerName] {
						playerInitMap[playerName] = true
						fmt.Println("initialized!")

						// Read Namespaces Env - Compile list of selected namespaces
						namespaces, _ := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})

						if len(passedNamespaces) > 0 {
							passedNamespacesList := strings.Split(passedNamespaces, ",")
							for _, ns := range namespaces.Items {
								for _, envns := range passedNamespacesList {
									if strings.EqualFold(ns.Name, envns) {
										selectednamespaces = append(selectednamespaces, ns.Name)
									}
								}
							}
							fmt.Print(passedNamespacesList)
							if len(selectednamespaces) < 4 { // if less than 4 specified, select until length is 4
								for _, ns := range namespaces.Items {
									if !Contains(selectednamespaces, ns.Name) {
										selectednamespaces = append(selectednamespaces, ns.Name)
										if len(selectednamespaces) == 4 {
											break
										}
									}
								}
							}
							fmt.Print(selectednamespaces)
						} else {
							for i := 0; i < 4; i++ {
								selectednamespaces = append(selectednamespaces, namespaces.Items[i].Name)
								fmt.Println("namespace ", selectednamespaces)
							}
						}

						fmt.Println("Selected namespaces: ", selectednamespaces)

						go LoopReconcile(player, clientset)
					}
				}
			})
		})

		// If a mob is killed by the player we do another check which entity is missing
		player.OnMobKilled(func(event *event.MobKilled) {
			fmt.Printf("mobkilled %d\n", event.MobType)
			var mobkilledtype string = strconv.Itoa(event.MobType)
			player.Exec("title @s actionbar @s killed mobtype "+mobkilledtype, nil)
			ReconcileMCtoKubeMob(player, clientset, event.MobType)
		})

		// Set up event handler for commands typed by player
		player.OnPlayerMessage(func(event *event.PlayerMessage) {
			fmt.Println(event.Message)
			if (strings.Compare(event.Message, "detect")) == 0 {
				player.Exec("title @s actionbar @s used detect!", nil)
			}

			if (strings.Compare(event.Message, "test")) == 0 {
				player.Exec("title @s actionbar @s used test!", nil)
			}

			if (strings.Compare(event.Message, "pos")) == 0 {
				GetPlayerPosition(player)
				SetNamespacesPositionByPos(initpos)
				//player.Position(func(pos mctype.Position) {
				//	fmt.Print(FloatToString(pos.X) + " " + FloatToString(pos.Y) + " " + FloatToString(pos.Z))
				//	//SetNamespacesPositionByPos(pos)
				//})
				//player.Rotation(func(rotation float64) {
				//fmt.Printf("  player rotation: %v\n", rotation)
				//})
				//player.Position(func(pos mctype.Position) {
				//	fmt.Printf(FloatToString(pos.X) + FloatToString(pos.Y) + FloatToString(pos.Z))
				//})

			}

			//if (strings.Compare(event.Message, "ns")) == 0 {
			//	namespaces, _ := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
			//	fmt.Printf(namespaces.ResourceVersion)
			//}

			if (strings.Compare(event.Message, "killall")) == 0 {
				fmt.Print("Killing entities")
				DeleteEntities(player)
			}

			// Initialize admin area
			if (strings.Compare(event.Message, "init")) == 0 {
				DeleteEntities(player)
				GetPlayerPosition(player)
				SetNamespacesPositionByPos(initpos)
				InitArea(player)
			}

			// Force sync if auto-init doesn't work
			if (strings.Compare(event.Message, "sync")) == 0 {
				fmt.Println("start syncing")
				go LoopReconcile(player, clientset)
			}
		})

	})
	server.OnDisconnection(func(player *mcwss.Player) {
		// Called when a player disconnects from the server.
		fmt.Println("Player ", player.Name(), " has disconnected")
	})

	// Run the server. (blocking)
	server.Run()
}
