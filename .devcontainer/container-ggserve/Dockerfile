FROM mcr.microsoft.com/devcontainers/go:1.25-trixie

# [Optional] Uncomment this section to install additional OS packages.
 RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
     && apt-get -y install --no-install-recommends ncbi-blast+ samtools \
     && apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/* # /tmp/library-scripts && rm -rf /tmp/downloaded_packages 

# [Optional] Uncomment the next lines to use go get to install anything else you need
# USER vscode
# RUN go get -x <your-dependency-or-tool>
# USER root


ENV GGTABLE_DATA=/data

# [Optional] Uncomment this line to install global node packages.
# RUN su vscode -c "source /usr/local/share/nvm/nvm.sh && npm install -g <your-package-here>" 2>&1
