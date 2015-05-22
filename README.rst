dmenu_hist
==========

dmenu run with history

dmenu wrapper for app launching, gives selection of executable files from path,
and does prefer entries previously launched by user, including possible arguments.

To install/obtain::
    # install go ~ https://golang.org/doc/install.html ~ or on Fedora like:
    dnf/yum/... install golang
    mkdir ~/go
    export GOPATH=$HOME/go

    # install dmenu_hist itself:
    go get github.com/queria/dmenu_hist

Written by Queria Sa-Tas <public@sa-tas.net> 2015, published under 3-Clause BSD Licence.
