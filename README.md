# Genome Table v.3.0

Genome Table v.3.0 is a complete rewrite of the P. Insidiosum genome database (v.2) designed for robust gene content comparison and comprehensive phylogenetic analysis. This version addresses previous scalability and accessibility limitations by incorporating key enhancements and enabling local deployment.

## Key Features

- **Improved Performance:** Built with Golang and SQLite to ensure seamless integration and prevent performance degradation.
- **Local Deployment:** Open-source and comes with a Docker image, allowing researchers to deploy and run the platform locally without reliance on external servers. **Gene Content Comparison Tools:** Includes BLAS, Heatmap, and Query Table functionalities to facilitate in-depth bioinformatics analyses.

## Repository Contents

- **Source Code:** Complete source code for Genome Table v.3.0.
- **Example Data:** Sample datasets to help you get started and test the platform.
- **Docker Setup:** Docker configuration files to streamline local deployment.

## Getting Started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed on your system.
- [Golang](https://golang.org/dl/) (if you wish to build or modify the source code directly).
- [SQLite](https://www.sqlite.org/index.html) (included in the Docker container for database operations).

### Installation & Setup

1. **Clone the Repository:**

   ```bash
   git clone https://github.com/yourusername/genome-table.git
   cd genome-table
   ```

2. **Using Docker:**

Build and run the Docker container:
   ```bash
   docker build -t genome-table .
   docker run -p 8080:8080 genome-table
   ```

3. **Local Setup Without Docker (Optional):**
   ```bash
   go build -o genome-table
   ./genome-table
   ```

## Usage
Soon

## Contributing
Soon

## License
This project is licensed under the MIT License. See the LICENSE file for details.

## Contact
For any questions or support, please open an issue in the repository or contact the project maintainer at your.email@example.com.
