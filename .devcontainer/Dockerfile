ARG GO_VER=1.22.4
ARG GO_ARCH=alpine

FROM golang:${GO_VER}-${GO_ARCH}

RUN  sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://alpine.cs.nycu.edu.tw|g' /etc/apk/repositories 

# Install Golang extension package tools
RUN go install github.com/ramya-rao-a/go-outline@v0.0.0-20210608161538-9736a4bde949    && \
    go install github.com/cweill/gotests/gotests@v1.7.0                                && \
    go install github.com/fatih/gomodifytags@v1.17.0                                   && \
    go install github.com/josharian/impl@v1.4.0                                        && \
    go install github.com/haya14busa/goplay/cmd/goplay@v1.0.0                          && \
    go install github.com/go-delve/delve/cmd/dlv@v1.25.2                               && \
    go install honnef.co/go/tools/cmd/staticcheck@v0.4.7                               && \
    go install golang.org/x/tools/gopls@v0.15.3


RUN apk add --no-cache make libc6-compat

RUN apk add --no-cache zsh-vcs tmux less && \
    sh -c "$(wget -O- https://github.com/deluan/zsh-in-docker/releases/download/v1.2.1/zsh-in-docker.sh)" -- \
    -t agnoster \
    -p git \
    -p z \
    -p terraform \
    -p https://github.com/zsh-users/zsh-autosuggestions \
    -a 'export TERM=xterm-256color' \
    -a 'export PAGER="less -FRX"' \
    -a 'export GIT_PAGER="less -FRX"' \
    -a 'export LESS="FRX"' \
    -a 'ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE="fg=2"' \
    -a 'AGNOSTER_CONTEXT_BG=52' \
    -a 'AGNOSTER_CONTEXT_FG=white' \
    -a 'AGNOSTER_DIR_BG=cyan' \
    -a 'AGNOSTER_DIR_FG=black' \
    -p https://github.com/zsh-users/zsh-completions

RUN sed -i '/## Main/i prompt_newline() { \n\
 if [[ -n $CURRENT_BG ]]; then\n\
   echo -n " %{%k%F{$CURRENT_BG}%}$SEGMENT_SEPARATOR\n\
%{%k%F{$CURRENT_BG}%}$SEGMENT_SEPARATOR"\n\
 else\n\
   echo -n " %{%k%}"\n\
 fi\n\
 echo -n " %{%f%}"\n\
 CURRENT_BG=''\n\
}' /root/.oh-my-zsh/themes/agnoster.zsh-theme && \
    sed -i 's/  prompt_hg/  prompt_hg\n  prompt_newline/' /root/.oh-my-zsh/themes/agnoster.zsh-theme

ARG INSTALL_PYTHON
RUN if [ "${INSTALL_PYTHON}" == "true" ]; then \
        apk add --no-cache python3 py3-pip ;\
        # install uv from offical script (alpine 3.20 does not have uv package)
        # TODO: uv will use /root/.local/bin/env, which conflict volume required by OpenCode
        # curl -LsSf https://astral.sh/uv/install.sh | sh ;\
    fi






