# FSM CLI Client User Guide

> *Author's Note*<br>
> The following instructions have been generated using the `gpt-3.5-turbo` OpenAI model,
> with minimal prompting, with the Go source code for the `cli/fsm-client.go` and `client`
> package as inputs.

The FSM CLI Client is a command-line interface tool that allows you to interact with a Finite State Machine (FSM) server. This user guide will provide instructions on how to use the CLI client and its available commands.

## Prerequisites
Before using the FSM CLI Client, please ensure that you have the following:

- **FSM CLI Client Binary:** Download the compiled binary of the FSM CLI Client for your operating system from the [Releases](https://github.com/massenz/go-statemachine/releases) page.

## Usage
To use the FSM CLI Client, follow the instructions below.

### Command Syntax
The general syntax for invoking the FSM CLI Client is as follows:

```
./fsm-cli [options] command [arguments]
```

### Options
The following options are available:

- `-insecure`: If set, TLS will be disabled (NOT recommended).
- `-addr`: The address (host:port) for the GRPC server. Default is `localhost:7398`.

### Available Commands
The FSM CLI Client supports the following commands:

- **send**: Sends an entity to the server.
- **get**: Retrieves an entity from the server.
- **version**: Displays information about the client and the connected server.

#### send Command
The `send` command allows you to send an entity to the FSM server. The entity should be provided as a YAML file.

**Command Syntax:**
```
./fsm-cli send [path_to_yaml_file]
```

**Examples:**
- Send an entity from a YAML file:
  ```
  ./fsm-cli send config.yaml
  ```

- Send an entity from standard input (stdin):
  ```
  ./fsm-cli send --
  ```
  *Note: Enter the YAML content in the command line and press Enter, followed by Ctrl+D on Linux/macOS or Ctrl+Z on Windows to signal the end of input.*

#### get Command
The `get` command allows you to retrieve an entity from the FSM server based on its kind and ID.

**Command Syntax:**
```
./fsm-cli get [kind] [id]
```

**Examples:**
- Retrieve a `Configuration` entity:
  ```
  ./fsm-cli get Configuration orders
  ```

- Retrieve a `FiniteStateMachine` entity:
  ```
  ./fsm-cli get FiniteStateMachine config-name/fsm-id
  ```

#### version Command
The `version` command displays information about the FSM CLI Client and the connected server.

**Command Syntax:**
```
./fsm-cli version
```

### Examples
Here are some examples of using the FSM CLI Client:

- Get the version information:
  ```
  ./fsm-cli version
  ```

- Send a `Configuration` entity from a YAML file:
  ```
  ./fsm-cli send config.yaml
  ```

- Retrieve a `FiniteStateMachine` entity:
  ```
  ./fsm-cli get FiniteStateMachine config-name/fsm-id
  ```

## YAML Example
Below is an example of a YAML file for a `Configuration` entity:

```yaml
# YAML example for CLI configuration

apiVersion: v1alpha
kind: Configuration

spec:
  name: orders
  version: v3

  states:
    - start
    - pending
    - shipped
    - end
  startingstate: start
  transitions:
    - from: start
      to: pending
      event: accept
    - from: pending
      to: shipped
      event: process
    - from: pending
      to: start
      event: review
    - from: start
      to: end
      event: cancel
    - from: shipped
      to: end
```

See other examples in the [`data`](https://github.com/massenz/go-statemachine/tree/main/data) folder.
