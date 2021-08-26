# Table of Contents

- [Table of Contents](#table-of-contents)
- [Conveniently switch between different Kubernetes clusters and namespaces](#conveniently-switch-between-different-kubernetes-clusters-and-namespaces)
- [kconfig goals](#kconfig-goals)
- [How kconfig works](#how-kconfig-works)
  - [Example](#example)
- [The kconfig.yaml file](#the-kconfigyaml-file)
- [The commands](#the-commands)
  - [kset - set up the environment to access a nickname](#kset---set-up-the-environment-to-access-a-nickname)
    - [Overrides on the kset command line](#overrides-on-the-kset-command-line)
  - [koff - clear any kconfig settings from the environment](#koff---clear-any-kconfig-settings-from-the-environment)
  - [kset nickname completion](#kset-nickname-completion)
  - [The kconfig version of the kubectl executable](#the-kconfig-version-of-the-kubectl-executable)
- [Installation](#installation)
  - [Use the Releases page](#use-the-releases-page)
  - [Use the go command to install](#use-the-go-command-to-install)
  - [Clone the repository and build it yourself](#clone-the-repository-and-build-it-yourself)
- [Miscellaneous](#miscellaneous)
  - [Does kconfig work with OpenShift?](#does-kconfig-work-with-openshift)
  - [Preventing an explosion of local kubectl configuration files](#preventing-an-explosion-of-local-kubectl-configuration-files)
  - [Do temporary configuration files need to be refreshed?](#do-temporary-configuration-files-need-to-be-refreshed)
  - [Is kconfig better than kalias?](#is-kconfig-better-than-kalias)
  - [How can I use a shortened command name like just "k"?](#how-can-i-use-a-shortened-command-name-like-just-k)
  - [Unexpected changes to the kubectl configuration file](#unexpected-changes-to-the-kubectl-configuration-file)

# Conveniently switch between different Kubernetes clusters and namespaces

When using several different Kubernetes configuration files, or contexts and namespaces, manually
typing `--kubeconfig`, `--context`, `--namespace`, or `--user` options on every `kubectl` command
can be tiresome.  The common solution is to create a `kubectl` context describing the desired
cluster, namespace, and user, and to set that as your current `kubectl` context.  But setting a
current context in a shared `kubectl` configuration file prevents different command sessions from
using different default settings.  This package attempts to address this and satisfy several other
requirements.

# kconfig goals

The kconfig package was created with these requirements in mind:

1. Use "pristine" `kubectl` configuration files.  When using a cloud provider like IBM Cloud, the
   proper `kubectl` configuration to use for a given cluster is usually created programmatically.
   In the case of IBM Cloud, the user issues a command like
   `ibmcloud ks cluster config -c cluster-name` to generate a `kubectl` configuration.  Depending on
   the cloud provider, this might add cluster, user, and context information to the default
   `~/.kube/config` file, or it might create a separate file and suggest that you set the
   `KUBECONFIG` environment variable to point to this file.

   After running the appropriate command, there's a temptation to edit the result and customize it
   as needed.  Maybe you'd like to change the namespace in the context, or replicate the context to
   have additional ones that reference different namespaces or users.  The problem is that sometimes
   you have to issue the cloud provider command again when tokens expire, or for other reasons, and
   your customizations can easily be overwritten.  So a goal of `kconfig` is that you never have to
   customize the generated `kubectl` configurations.  The `kconfig` commands will _never_ modify
   your existing `kubectl` configuration files.

2. Allow combinations of `--kubeconfig`, `--context`, `--namespace`, and `--user` options to be
   referred to with short nicknames.  This is needed because at least some cloud providers generate
   long, impractical-to-type-or-remember `kubectl` configuration file names or context names.  The
   user should be able to refer to their favorite combinations with a simple nickname that is
   meaningful to them.

3. Each command-line session ought to be able to use a different nickname, or in general, a
   different combination of `kubectl` options, without any extra typing, and without affecting any
   other command-line sessions.

4. Since different command-line sessions can be using different Kubernetes clusters, the shell
   prompt should (optionally) be updated to mention the nickname, to help the user remember which
   nickname is targeted by each command-line session.

5. When using multiple Kubernetes clusters, they can be at different server versions.  Kubernetes
   supports a plus-or-minus difference of only _one_ version between the client and the server.
   When using multiple clusters, therefore, you often have to install several versions of the
   `kubectl` executable and remember to use the appropriate version for each cluster.  A goal of
   `kconfig` is to make this automatic.  Your `kconfig` nickname can optionally specify which
   `kubectl` executable to use for the nickname.

6. The implementation shouldn't require using a command other than `kubectl`, like a shell alias or
   different frontend to `kubectl`.  Otherwise third-party utilities that use `kubectl` won't work.
   A shell alias, like that created by the older [kalias](https://github.com/jphx/kalias)
   package,
   [also has problems](https://github.com/jphx/kconfig/wiki/Comparing-kconfig-with-kalias).

# How kconfig works

To use `kconfig`, create a file called `~/.kube/kconfig.yaml`.  It can specify several
[preferences](#the-kconfigyaml-file), such as the default `kubectl` executable to use, and a list of
nicknames, expressed as combinations of the `kubectl` options `--kubeconfig`, `--context`, `-n`
(`--namespace`), and `--user`.  For example:

```yaml
nicknames:
  dev: --context dev
  dev-app: --context dev --namespace application1
  dev-admin: --context dev --user admin
  stage: --kubeconfig /home/jph/clusters/staging-cluster -n application1
  prod: kubectl-1.20.2 ---kubeconfig /home/jph/clusters/production-cluster -n application1
  modern: oc --context default/c114-e-uxxxx --namespace application1
```

Once [installed](#installation), to switch the current command-line session to a given nickname,
you run the **kset** command (shell function, really) specifying the nickname:
```bash
kset dev
```
This does the following things:

1. It creates a temporary session-specific `kubectl` configuration file that puts the desired
   options into effect.
2. It sets the `KUBECONFIG` environment variable to point to this file, followed by either the
   default file, or the file referenced in the `--kubeconfig` option of the nickname.
3. It changes (by default) the `PS1` shell variable to include the nickname in the command prompt.
4. It sets the `_KCONFIG_KUBECTL` environment variable to the name of the `kubectl` executable to be
   used for this nickname.

To use the settings in the nickname, the user now simply types `kubectl` commands like normal.  If
you choose to install the **kubectl** command that's packaged with `kconfig`, then it will `exec`
the `kubectl` executable that should be used for the nickname, which it knows from reading the
`_KCONFIG_KUBECTL` environment variable.  If you choose not to install this version of **kubectl**,
you'll lose that feature and one other that is
[described later](#the-kconfig-version-of-the-kubectl-executable).

To stop using this nickname, and reset your environment to the way it was before you ran **kset**,
run the **koff** command (again, really a shell function).

## Example

An example can make this clearer:
```
$ kset dev

(dev) $ echo $KUBECONFIG
/tmp/kconfig/sessions/812129604.yaml:/home/jph/.kube/config

(dev) $ cat /tmp/kconfig/sessions/812129604.yaml
apiVersion: v1
kind: Config
preferences: {}
clusters: null
contexts: null
current-context: dev
users: null

(dev) $ kubectl version --short
Client Version: v1.19.2
Server Version: v1.19.12+IKS
```

In this example, the session-local `kubectl` configuration file merely contains a `current-context`
section to select a different context from the default file `/home/jph/.kube/config`.  Any `kubectl`
command now uses this context by default.

Since no default `kubectl` executable was provided and none is provided in the nickname definition,
invocations of the **kubectl** program that comes with `kconfig` will `exec` the normal `kubectl`
executable, which should exist later in the `PATH` than the `kconfig` version of **kubectl**.  In
this example, the `kubectl` executable is at version `v1.19.2`.

Now we continue on with the same example:
```
(dev) $ kset prod

(prod) $ echo $KUBECONFIG
/tmp/kconfig/sessions/812129604.yaml:/home/jph/clusters/production-cluster

(prod) $ cat /tmp/kconfig/sessions/812129604.yaml
apiVersion: v1
kind: Config
preferences: {}
clusters: null
contexts:
- context:
    cluster: the-prod-cluster
    namespace: application1
    user: the-jph-identity
  name: kconfig_context
current-context: kconfig_context
users: null

(prod) $ kubectl version --short
Client Version: v1.20.2
Server Version: v1.20.2+IKS

(prod) $ koff
$ echo $KUBECONFIG

$
```

In this example, the session-local `kubectl` configuration file is slightly more complicated because
the nickname definition isn't a simple `--context` reference, so no existing context necessarily
provides what the nickname is asking for, namely overriding the default namespace.  So this
session-local `kubectl` configuration file defines a new context, derived from the default context
in the referenced configuration file, and makes it the current context for the session.  Note that
since the nickname included a `--kubeconfig` option referencing a different `kubectl` configuration
file, that file name appears in the `KUBECONFIG` path after the session-local file instead of the
default.  This is necessary so that the cluster name and user identity can be found in that file,
and is useful anyway in case the user wants to reference another context using `--context` options
on the `kubectl` command line.

The other difference to notice is that the `kubectl version` command printed a different client
version.  The reason is that this nickname describes that the `kubectl-1.20.2` executable should be
used for `kubectl` commands for this nickname, so when the `kubectl` executable ran, it `exec`ed
this executable instead of the default.

When you're done, run the `koff` shell function to remove the session-local `kubectl` configuration
file, unset the `KUBECONFIG` environment variable, and restore the command prompt.

# The kconfig.yaml file

The format of the `~/.kube/kconfig.yaml` file is the following:

```yaml
preferences:
  # The name (with or without a path) of the kubectl executable to use if the nickname definition
  # doesn't explicitly provide one.  If not specified, the default is "kubectl".
  default_kubectl: kubectl-1.xx.y

  # Indicates whether or not the kset command function modifies the PS1 shell variable, to change the
  # shell prompt after a kset command.  E.g., "kset dev" would prefix the prompt with this:  (dev)
  # If unspecified, the default is true.
  change_prompt: false

  # Indicates whether or not the kset shell function includes "overrides" in the shell prompt, if
  # overrides are specified on the "kset" command.  E.g., "kset dev -n foo" would put this in the
  # prompt:  (dev[ns=foo])
  # If unspecified, the default is true.
  show_overrides_in_prompt: false

  # Says whether or not kset will look in the ~/.kube/kalias.txt file as a source of nicknames.
  # This is to make it easier to migrate from the older kalias utility.  The default is false,
  # unless the ~/.kube/kconfig.yaml file doesn't exist, in which cases it's true.
  read_kalias_config: true

  # The default KUBECONFIG environment variable setting to be used.  If not specified, it defaults
  # to the empty string, which kubectl interprets as "~/.kube/config".  Specify this if your
  # "normal" kubectl configuration file (or files) is different than "~/.kube/config".
  # This will cause kset to search this file path instead of the default when looking up context
  # information, and koff will restore the KUBECONFIG environment variable to this value instead of
  # unsetting it.
  base_kubeconfig: /home/jph/cluster-info/file1.yaml:/home/jph/cluster-info/file2.yaml

# nicknames is a map of nicknames to definitions.  A definition is a string that optionally starts
# with the name of the kubectl executable to use for this nickname, followed by any of these
# options, whose meaning is the same as for the kubectl command:
#  --kubeconfig FILE
#  --context CONTEXT-NAME
#  -n NAMESPACE-NAME (--namespace NAMESPACE-NAME)
#  --user USER-NAME
# The first token of the string is considered to be the executable name if it doesn't start with
# a dash (-).
nicknames:
  nick1: defn1
  nick2: defn2
  nick3: defn3
```

# The commands

Once you [install](#installation) kconfig and run the setup script in the current shell -- most
likely from your `~/.bashrc` script -- a number of shell functions will then be available that you
use like commands.  They're provided as shell functions so that they can manipulate settings of the
current shell, like the environment variable `KUBECONFIG` and the shell variable `PS1`.  The shell
functions themselves are simple.  The implementations generally invoke the `kconfig-util` command to
do the real work.

These are the shell functions intended for users:

- **kset**: Switch to the Kubernetes cluster selected by the given nickname.
- **koff**: Clear any settings from the current command shell that were made by **kset**.

These are describe in detail in the following sections.

## kset - set up the environment to access a nickname

The **kset** command is the one you'll use most often.  It makes changes to the command-line session
in which you type the command to configure **kubectl** to access the Kubernetes cluster, context,
namespace, etc., that you describe in your nickname definition. The syntax is:

    kset nickname [options]

where `nickname` is one of the nicknames from your `kconfig.yaml` file (or `kalias.txt` file if
`kconfig` is configured to look there also for nickname definitions).

In the simplest form, you'll just type `kset nickname` to create a session-local `kubectl`
configuration file for the current command session that accesses the Kubernetes cluster, etc, that
is described by the given nickname.  The `KUBECONFIG` environment variable is set to include the
session-local `kubectl` configuration file in the `kubectl` search path.  Unless you've disabled it in
the preferences, your shell prompt will be modified to include the nickname in the prompt, so you'll
know what nickname each command session is currently accessing.

For example:
```
$ kset dev
(dev) $ echo $KUBECONFIG
/tmp/kconfig/sessions/812129604.yaml:/home/jph/.kube/config
```

### Overrides on the kset command line

It's also possible to override selected settings in the `kubectl` context by adding `kubectl`
options to the **kset** command line.  You can execute `kset --help` to see the options help.
The supported options are:

        --kubeconfig=FILE    Path to the kubectl config file to use.  If not specified, the default
                             is the value from the nickname definition, or ~/.kube/config if none
                             is provided there.
        --context=NAME       The name of the context to use from the kubectl config file.  If not
                             specified, the default is the value from the nickname definition, or
                             the context if none is provided there.
    -n, --namespace=NAME     The namespace to use.  If not specified, the value from the nickname
                             definition is used, or if none is provided there, the namespace
                             associated the specified or default context.
        --user=NAME          The user name to use.  If not specified, the value from the nickname
                             definition is used, or if none is provided there, the user
                             associated the specified or default context.

As an example of how to use override options, assume you have a nickname like the following, that
specifies a particular context and namespace:
```yaml
dev-app: --context dev --namespace application1
```

If you run:
```bash
kset dev-app
```
the session-local `kubectl` configuration file that's created will have`namespace: application1` in
the generated context.  But if you run:
```bash
kset dev-app -n application2
```
then the session-local `kubectl` configuration file will have `namespace: application2` in the
generated context.  This "override" syntax is useful for accessing namespaces you don't commonly
access, but still have an occasional need to access, but don't want to bother creating a separate
`kconfig` nickname for.  It's also useful to be able to override the user with an override option.
Overriding the `kubectl` context and `kubectl` configuration file are also possible, but probably
less useful.

**Remember, the changes effected by `kset` will affect _only_ the command line session in which you
enter the kset command.**

You can issue **kset** commands consecutively without running **koff** in between.  It will change
the session-local `kubectl` configuration file to reference the new nickname.

## koff - clear any kconfig settings from the environment

The **koff** command is used to undo the effects of the **kset** command.  It will unset the
`KUBECONFIG` environment variable, or set it to the `base_kubeconfig` path if you specified one in
the `kconfig.yaml` preferences.  It will restore the `PS1` shell variable to the value it had before
the **kset** command modified it.  It will also delete the session-local `kubectl` configuration
file that was created for this command-line session.

You can issue **kset** commands to switch to a new nickname without running **koff** in between.

## kset nickname completion

When you start to have a large number of `kconfig` nicknames defined, you might not be able to
easily remember their names.  The `kset` command therefore supports Bash shell completion.
E.g., if you type `kset dev` and then hit tab once, the nickname will be auto-completed if it's
unique.  If it's not unique, hit tab twice to see all the nicknames that start with that prefix.

## The kconfig version of the kubectl executable

The `kconfig` package includes a program called **kubectl**.  This program, of course, has the
_same_ name as the normal program used to access Kubernetes clusters.  The primary reason for this
is to support using different `kubectl` executable programs for different clusters, which can be
required if the clusters are at widely different Kubernetes server levels, since Kubernetes requires
that the `kubectl` client program be within one minor version of the server level.

Don't let the fact that this program has the same name as the real `kubectl` program worry you.
It's actually a very simple program that merely examines the `_KCONFIG_KUBECTL` environment variable
that is set by the **kset** command.  The value is the name of the real `kubectl` executable to use
for the current session.  The `kconfig` version of **kubectl** searches for this name in the `PATH`
and does an `exec` operation to run it.  The `exec` operation replaces the process image with the
target version of `kubectl`, which executes your command.  If the environment variable isn't set,
it searches for the default `kubectl` executable name specified in the `kconfig.yaml` preferences,
or if that's not specified, the name `kubectl` (skipping itself), and then runs that program.

The second useful feature of the **kubectl** program that is distributed with `kconfig` is that it
accepts, _as the first option only_, the **-k** (**--kconfig**) option to provide a `kconfig`
nickname to use for the command.  This will be used instead of any presently-configured `kconfig`
nickname, if there is one, _for this single command_.  No lasting changes will be made to the
current command-line environment when this option is used.  For example:

```bash
kubectl -k dev get pods
kubectl --kconfig dev get pods
```

# Installation

The `kconfig` package downloaded from the
[Releases](https://github.com/jphx/kconfig/releases/latest) page consists of these files:

1. A file to run from shell initialization to create the shell functions.  This file is in the
   `setup` directory of the package.  Presently `kconfig` supports only the
   [Bash](https://www.gnu.org/software/bash/) shell, but contributions to add support for additional
   shells would be welcome.  You can put the setup script anywhere on your system.  Source it from
   your shell initialization file.  For `bash`, you might include lines like the following in your
   `~/.bashrc` file so that it runs only for interactive shells:

   ```bash
   if [[ $- == *i* ]]; then
      # Initialize kconfig shell functions and auto completion
      . /path/to/kconfig/setup/kconfig-setup.sh
   fi
   ```

2. The **kconfig-util** program that's use by the shell functions to perform the real work.  Put
   this program anywhere in your `PATH` so that it's available when the shell functions need it.

3. The **kubectl** program that is a frontend to the official Kubernetes `kubectl` program.  If the
   name of this program makes you uncomfortable, it's not necessary to install it, but you'll lose
   the following features:

   1. The ability to use different versions of the `kubectl` program for different clusters, to
      conform with the rule that the `kubectl` client must be within one minor version of the
      Kubernetes server.
   2. The ability to run "one off" commands using the **-k** (**--kconfig**) option of this
      **kubectl** program, which creates a temporary `kubectl` configuration file that is used for
      just this command execution.

   To install the **kubectl** program, put it in your `PATH`.  If you want it to be able to forward
   the execution to the official Kubernetes program that you already have installed with the name of
   `kubectl`, make sure to put the `kconfig` version of **kubectl** earlier in the `PATH` than the
   Kubernetes version.  The `kconfig` version of **kubectl** is smart enough to skip itself when
   searching for the target program name, so it can find a target program by the same name.

You can get the executable programs in the following ways.

## Use the Releases page

Point your browser at https://github.com/jphx/kconfig/releases/latest and download the tar file
for the operating system that you're using.  Tar files are provided for Linux and MacOS.

Expand the tar file someplace on your filesystem.  Copy or move the executable programs into your
`PATH`.  Make sure the provided **kubectl** program is in a directory earlier in the `PATH` than the
Kubernetes version of `kubectl`.   Find the best setup shell script from the `setup` subdirectory
and call it from your shell initialization file as described [above](#installation).  The available
files are:

 - The Bash shell: kconfig-setup.sh

If no tar file suitable for your operating system is provided, use one of the following alternative
installation approaches.

## Use the go command to install

If you have a [Go](https://golang.org/) development environment set up on your system, an
alternative to downloading the tar file from the Releases page and manually copying the executable
programs to some directory in your `PATH` is to run:

```bash
go install github.com/jphx/kconfig/...@latest
```

The executable programs will be installed into your `$GOBIN` directory, which you should include in
the `PATH`.  Make sure the `$GOBIN` directory is earlier in your `PATH` than where the Kubernetes
version of `kubectl` is installed.

Don't forget to fetch the setup shell script and source it from your shell initialization file
(e.g., `~/.bashrc`).

## Clone the repository and build it yourself

If you have a Go development environment set up, you also have the option of cloning the git
repository and running `make install`.  This builds installs the two executable programs into your
`$GOBIN` directory.  Again, make sure the `$GOBIN` directory is earlier in your `PATH` than where
the Kubernetes version of `kubectl` is installed.

```bash
git clone https://github.com/jphx/kconfig.git
cd kconfig
make install
```

Don't forget to source the setup shell script from your shell initialization file (e.g., `~/.bashrc`).

# Miscellaneous

The following sections address some miscellaneous questions that might arise.

## Does kconfig work with OpenShift?

The `oc login` command that you use to access an OpenShift cluster updates your `~/.kube/config`
file to describe the cluster and user, and adds a context for the new cluster, and then makes it the
current context.  This is an example of a cloud provider command that creates configuration
information.  The `kconfig` package can definitely be used to access OpenShift clusters.  Here are
some suggestions that may be useful when using OpenShift:

1. You might consider specifying the `oc` executable as the "kubectl" program name in the nickname
   definitions for your OpenShft clusters.  E.g.,
   ```yaml
   nicknames:
     dev: oc --context default/c114-e-uxxxx --namespace application1
   ```
   This will cause the **kubectl** command to forward all invocations to `oc` instead of the default
   `kubectl` program.  If _all_ the clusters you access are OpenShift, you can set the
   `default_kubectl` preference to `oc` so that the `oc` program is used by default instead of
   `kubectl`.  This isn't actually necessary, though, since the `kubectl` command can access
   OpenShift clusters as well.  You don't need to do a lot of `oc project` commands, either, to
   switch namespaces.

2. If you like typing `oc` all the time instead of `kubectl`, you can copy or rename the version of
   **kubectl** that is distributed with `kconfig` to the name `oc`, installing it in the `PATH`
   earlier than the official `oc` program.

3. If you used **kset** in your command-line session, run **koff** before using `oc login` to
   refresh your configuration.  The reason is that `oc login` appears to update the _first_ file
   mentioned in the `KUBECONFIG` environment variable instead of the last, so instead of updating
   the "permanent" configuration file, it would update the temporary one instead.  This probably
   isn't what you want.

## Preventing an explosion of local kubectl configuration files

Temporary `kubectl` configuration files are created on two occasions:

1. When the **kset** command is executed.
2. When the **kubectl** command is executed with a leading **-k** (**--kconfig**) option.

When the file is created by **kset**, it has an essentially random filename and resides in the
`/tmp/kconfig/sessions` directory (or possibly a directory other than `/tmp` on your system).  This
file is deleted when the **koff** command executes.  Consecutive **kset** commands reuse the same
file.

If you exit the command-line shell where you ran **kset** without running **koff**, this file won't
be deleted.  It should eventually be deleted by your system's normal temporary file cleanup
procedures, though.  If these leftover files are a problem, try to remember to run **koff** before
exiting your command-line session.

When the temporary `kubectl` configuration file is created by the **kubectl** command because the
**-k** (**--kconfig**) option is specified, the file is named after the nickname, and resides in the
`/tmp/kconfig/nicks` directory.  This file can't be deleted by **kubectl** because that utility uses
`exec` to transfer control to the target `kubectl` executable.  It therefore has no opportunity to
clean up the file.  However, since the file is named for the nickname, you'll never accumulate more
of them than you have nicknames.  If they are unused, these files should also eventually be deleted
by your system's normal temporary file cleanup procedures.

## Do temporary configuration files need to be refreshed?

You might wonder whether it's necessary to run **kset** again after using your cloud provider's
command to refresh your Kubernetes cluster configuration.  Generally the answer is no.  Remember
that the temporary configuration file has only a context in it (and sometimes just a
`current-context` specification).  The context refers to a cluster name, a namespace name, and a
user name.  Unless your cloud provider command changes one of these in your base configuration
files, it shouldn't be necessary to run **kset** after updating them.  If, however, one of these
things change, then yes, you should run the **kset** command again to refresh the local
configuration file.

**Beware:** Some cloud provider commands update the file referenced by the _first_ entry in the
`KUBECONFIG` environment variable instead of the last.  The `oc login` command appears to behave
this way.  If the command you use to refresh your configuration is one of these, be sure to run
**koff** before executing the command that refreshes your configuration, so the command won't update
the temporary file instead of the file containing the long-term configuration.

## Is kconfig better than kalias?

The older [kalias](https://github.com/jphx/kalias) utility is a small program that attempted to
provide a similar capability, but its simplicity kept it from working in a variety of circumstances.
A discussion of the benefits of `kconfig` is available in
[Comparing kconfig with kalias](https://github.com/jphx/kconfig/wiki/Comparing-kconfig-with-kalias).

## How can I use a shortened command name like just "k"?

The `kconfig` package is intentionally designed to allow the user to run the normal **kubectl**
command to access a Kubernetes cluster.  This ensures that third-party shell scripts can target the
desired environment.  There isn't a different frontend program, and there isn't anything like a
shell alias to use.  But repeately typing `kubectl` all day can be inefficient, so some users prefer
to use a shortened name like `k`.  You can certainly do this.  I would recommend creating a `k`
symbolic link in a directory in your `PATH`, linking to the **kubectl** program that is distributed
with the `kconfig` package.  That way you can use `k` or `kubectl` when you type commands, and any
other programs that run the full `kubectl` name work fine as well.

## Unexpected changes to the kubectl configuration file

You may notice that the current context of your `~/.kube/config` file occasionally gets changed to
`kconfig_context`, which is the context name that `kconfig` uses for the temporary context it
creates in a session-local `kubectl` configuration file.  This change to the `~/.kube/config` file
is _not_ being made by `kconfig`.  Investigation reveals that it seems to be done by the `kubectl`
utility during normal commands (e.g., `kubectl get pods`) when using the `oidc` authentication
provider.  Sometimes `kubectl` finds it necessary to refresh the authentication token for the user.
But instead of modifying just the `user` section of the configuration file, it also seems to copy
the current context from the first file in the `KUBECONFIG` path into the `~/.kube/config` file.  I
don't know if there's a good reason for this.  It could be bug.  I don't know how to prevent it.

If you rely on the current context setting in your `~/.kube/config` file, then this behavior is
troublesome.  But if you always use `kset` to set your current context before running `kubectl`
commands, then the current context setting in the `~/.kube/config` file will never be used, so this
behavior isn't an issue.
