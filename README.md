# Genome Table v.3

Genome Table v.3 is a complete rewrite of the P. Insidiosum genome database (v.2) designed for robust gene content comparison and comprehensive phylogenetic analysis. This version addresses previous scalability and accessibility limitations by incorporating key enhancements and enabling local deployment.

## Key Features

- **Improved Performance:** Develop in  Golang with SQLite backend for speed and efficiency.
- **Local Deployment:** Fully open-source and Dockerized, enabling easy local usage without depending on external servers.
- **Gene Content Analysis Tools:** BLAST, Heatmap Viewer, Query table

## Repository Contents

- **Source Code:** Complete source code for Genome Table v.3.0.
- **Example Data:** Sample datasets to help you get started and test the platform.
- **Docker Setup:** Docker configuration files to streamline local deployment.

## Getting Started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed on your system.

### Installation

You can run Genome Table v3.0 in two ways:

1. **Use prebuit Docker image**

   ```bash
     docker run -p 8080:8080 \
       --mount type=bind,source="$(pwd)/data,target=/data" \
       docker.io/preechapat/ggtable:3.0.0
   ```
   Access the web app at: localhost:8080.

2. **Building docker image from source**

   ```bash
   git clone https://github.com/PreechaPat/ggtable
   cd ggtable
   docker build . -t preechapat/ggtable:3.0.0-dev
   docker run -p 8080:8080 \
     --mount type=bind,source="$(pwd)/data,target=/data" \
     preechapat/ggtable:3.0.0-dev
   ```

## Usage

After running the container (via either method), the application will be accessible on http://localhost:8080.

## Contributing
Contributions are welcome! Guidelines and instructions will be added soon.

## License
This project is licensed under the MIT License. See the LICENSE file for details.

## Contact
For any questions or support, please open an issue or contact the project maintainer at preecha.pat@mahidol.ac.th.
