# Cloud Bin

CloudBin is a distributed object storage system inspired by systems like Amazon S3. It is built using a microservices architecture with Go and Docker, designed for learning and demonstrating core concepts of distributed storage systems.

## Features
- Object storage (upload, download, delete files)
- Distributed storage across multiple nodes
- Replication for fault tolerance
- Metadata management (ACLs, tags, etc.)
- Authentication via a dedicated service

## Endpoints
### Auth endpoints
- api/v1/login
- api/v1/register
- api/v1/logout
- api/v1/get-user
- api/v1/update-user
- api/v1/delete-user
- api/v1/get-user-files

### File Management endpoints
- api/v1/upload-file
- api/v1/download-file
- api/v1/delete-file