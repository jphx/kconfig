# The kset shell function is intended to be run as:  kconfig name [override-options]
# Where name is an entry in the kconfig.yaml file, where each entry has
# the format:
#
# kubectl-options
#  or
# kubectl-executable-name kubectl-options
#
# E.g.,
# stage=--context staging
#  or
# stage=kubectl-1.21.0 --context staging --namespace testing
#
# The script creates or updates a session-local kubectl config file to provide a current context
# that is configured as specified.  The KUBECONFIG environment variable is updated to describe the
# appropriate search path, starting with the session-local config file.  The _KCONFIG_KUBECTL env
# var is to set name the kubectl executable to use for this nickname, to allow the kubectl
# executable supplied with kconfig to forward the operation to the selected kubectl executable.  If
# an executable name isn't provided, "kubectl" is used.

# The user can type "koff" to undo the effects of kconfig and to restore the command prompt.
function koff() {
   # Restore the shell prompt.
   if [[ -n "$_KCONFIG_OLD_PS1" ]]; then
      PS1="$_KCONFIG_OLD_PS1"
      unset _KCONFIG_OLD_PS1
   fi

   # Remove any session-local kubectl configuration file and unset or restore the KUBECONFIG env var.
   if [[ -n "$KUBECONFIG" ]]; then
      eval $(kconfig-util koff "$@")
   fi

   # More cleanup
   unset _KCONFIG_KUBECTL TELEPORT_PROXY
}

# The main kset command.  See the prologue comments.
function kset() {
   # Run the service utility to create the session-local config file.  Evaluate any statements it
   # sends to standard output, which we expect are to set environment variables.
   local _KP
   eval $(kconfig-util kset "$@")

   # kconfig-util sets the _KP variable with the shell prompt info
   if [[ -n "$_KP" ]]; then
      if [[ -z "$_KCONFIG_OLD_PS1" ]]; then
         _KCONFIG_OLD_PS1="$PS1"
         PS1="($_KP) $PS1"
      else
         PS1="($_KP) $_KCONFIG_OLD_PS1"
      fi
   fi
}

# A bash command completion function, to complete alias names.
function _kconfig_cmpl {
   local -i idx=0
   for name in $(kconfig-util complete "$2"); do
      COMPREPLY[$idx]="$name"
      idx=$idx+1
   done
}

complete -F _kconfig_cmpl kset

if [[ "$1" == "clean" ]]; then
   koff
   unset kset
   unset _kconfig_cmpl
   unset koff
   complete -r kset
fi
