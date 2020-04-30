package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli"
)

const alphanumericRegex = "^[a-zA-Z0-9_-]*$"
const dbFile = "~/.golog"

var dbPath, _ = homedir.Expand(dbFile)
var repository = TaskCsvRepository{Path: dbPath}
var transformer = Transformer{}
var commands = []cli.Command{
	{
		Name:         "start",
		Aliases:      []string{"begin", "b"},
		Usage:        "Start tracking a given task",
		Action:       Start,
		BashComplete: AutocompleteTasks,
	},
	{
		Name:         "stop",
		Aliases:      []string{"end", "e"},
		Usage:        "Stop tracking a given task",
		Action:       Stop,
		BashComplete: AutocompleteTasks,
	},
	{
		Name:         "status",
		Usage:        "Give status of task",
		Action:       Status,
		BashComplete: AutocompleteTasks,
	},
	{
		Name:    "clear",
		Aliases: []string{"clean", "c"},
		Usage:   "Clear all data",
		Action:  Clear,
	},
	{
		Name:    "list",
		Aliases: []string{"l"},
		Usage:   "List all tasks",
		Action:  List,
	},
}

// Start a given task
func Start(context *cli.Context) error {
	identifier := context.Args().First()
	if !IsValidIdentifier(identifier) {
		return invalidIdentifier(identifier)
	}

	// check if this tasks is already active then do nothing
	if active, err := IsActive(identifier); err != nil {
		return err
	} else if active {
		return errors.New(identifier + " (running)")
	}

	// stop all active tasks
	if err := StopAll(); err != nil {
		return err
	}

	err := repository.save(Task{Identifier: identifier, Action: "start", At: time.Now().Format(time.RFC3339)})

	if err == nil {
		fmt.Println("Started tracking ", identifier)
	}
	return err
}

// Stop a given task
func Stop(context *cli.Context) error {
	identifier := context.Args().First()

	if len(identifier) == 0 {
		// stop all active tasks
		StopAll()
		return nil
	}

	if !IsValidIdentifier(identifier) {
		return invalidIdentifier(identifier)
	}

	err := repository.save(Task{Identifier: identifier, Action: "stop", At: time.Now().Format(time.RFC3339)})

	if err == nil {
		fmt.Println("Stopped tracking ", identifier)
	}
	return err
}

// Status display tasks being tracked
func Status(context *cli.Context) error {
	identifier := context.Args().First()
	if !IsValidIdentifier(identifier) {
		return invalidIdentifier(identifier)
	}

	tasks, err := repository.load()
	if err != nil {
		return err
	}
	transformer.LoadedTasks = tasks.getByIdentifier(identifier)
	tasksTimes, _ := transformer.Transform()
	fmt.Println(tasksTimes[identifier])
	return nil
}

// List lists all tasks
func List(context *cli.Context) error {
	var err error
	transformer.LoadedTasks, err = repository.load()
	if err != nil {
		return err
	}

	var uitems []string
	for _, task := range transformer.LoadedTasks.Items {
		unique := true
		for _, u := range uitems {
			if u == task.getIdentifier() {
				unique = false
				break
			}
		}
		if unique {
			uitems = append(uitems, task.getIdentifier())
		}
	}

	list, total := transformer.Transform()

	for _, identifier := range uitems {
		fmt.Println(list[identifier])
	}

	if len(uitems) > 0 {
		fmt.Println()
		fmt.Println("Total: ", total)
	} else {
		fmt.Println("Time didn't tracked")
	}
	return nil
}

// ActiveTasks active tasks list
func ActiveTasks() (list []string, err error) {

	tasks, err := repository.load()
	if err != nil {
		return
	}

	status := make(map[string]bool)
	order := make([]string, 0)

	for _, task := range tasks.Items {
		switch task.getAction() {
		case "start":
			if _, in := status[task.getIdentifier()]; !in {
				order = append(order, task.getIdentifier())
			}
			status[task.getIdentifier()] = true
		case "stop":
			if _, in := status[task.getIdentifier()]; !in {
				order = append(order, task.getIdentifier())
			}
			status[task.getIdentifier()] = false
		}
	}

	for _, identifier := range order {
		if status[identifier] {
			list = append(list, identifier)
		}
	}

	return
}

// IsActive check if task is active
func IsActive(identifier string) (bool, error) {
	list, err := ActiveTasks()
	if err != nil {
		return false, err
	}
	for _, i := range list {
		if i == identifier {
			return true, nil
		}
	}
	return false, nil
}

// StopAll stops all active tasks
func StopAll() error {
	// check if we have started tasks we have to stop them
	activeList, err := ActiveTasks()
	if err != nil {
		return err
	}
	// stop active tasks
	for _, task := range activeList {
		if err := repository.save(Task{Identifier: task, Action: "stop", At: time.Now().Format(time.RFC3339)}); err != nil {
			return err
		}
		fmt.Println("Stopped tracking ", task)
	}
	return nil
}

// Clear all data
func Clear(context *cli.Context) error {
	err := repository.clear()
	if err == nil {
		fmt.Println("All tasks deleted")
	}
	return err
}

// AutocompleteTasks loads tasks from repository and show them for completion
func AutocompleteTasks(context *cli.Context) {
	var err error
	transformer.LoadedTasks, err = repository.load()
	// This will complete if no args are passed
	//   or there is problem with tasks repo
	if len(context.Args()) > 0 || err != nil {
		return
	}

	for _, task := range transformer.LoadedTasks.Items {
		fmt.Println(task.getIdentifier())
	}
}

// IsValidIdentifier checks if the string passed is a valid task identifier
func IsValidIdentifier(identifier string) bool {
	re := regexp.MustCompile(alphanumericRegex)
	return len(identifier) > 0 && re.MatchString(identifier)
}

func checkInitialDbFile() {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		os.Create(dbPath)
	}
}

func main() {
	// @todo remove this from here, should be in file repo implementation
	checkInitialDbFile()
	app := cli.NewApp()
	app.Name = "Golog"
	app.Usage = "Easy CLI time tracker for your tasks"
	app.Version = "0.1"
	app.EnableBashCompletion = true
	app.Commands = commands

	var err error

	switch len(os.Args) {
	case 1:
		// without arguments default List
		err = List(nil)
	case 2:
		// if first argument is not a command
		// use it as task name to start
		if nil == app.Command(os.Args[1]) {
			err = app.Run([]string{os.Args[0], "start", os.Args[1]})
		} else { // default
			err = app.Run(os.Args)
		}
	default:
		err = app.Run(os.Args)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func invalidIdentifier(identifier string) error {
	return fmt.Errorf("identifier %q is invalid", identifier)
}
