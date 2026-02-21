Mario's Notes (using agent as base layer)
# Plan for (Inventory + Transport Abstractions)


## Define core domain types

AgentID, Version, Manifest, AssetRef, Checksum, Tag, Publisher.
Manifest includes metadata, dependencies, and asset references.


## Define Inventory (registry/index) interface

* Responsibilities: discovery, metadata lookup, versions.
  * Interface sketch:
    *  Search(query, filters) -> []AgentSummary
    *  ListByPublisher(publisher) -> []AgentSummary
    *  GetManifest(agentID, version) -> Manifest
    *  ListVersions(agentID) -> []Version
    *  PublishManifest(manifest) -> error
    *  Storage-agnostic, pure metadata.


## Define Transport (artifact storage) interface

* Responsibilities: upload/download blobs (agent TOML, assets).
  * Interface sketch:
    * Put(ctx, key, io.Reader, checksum) -> AssetRef
    * Get(ctx, key) -> io.ReadCloser
    * Exists(ctx, key) -> bool
    * The key can be derived from AgentID + Version + path.
    * Supports verification via checksum.

## Define Hub interface that composes both

* Hub uses Inventory + Transport.
  * Operations (client-facing):
    * Search, Install, Publish, ListInstalled, Update.

## Test hub implementation

* In-memory or filesystem Inventory + Transport.
* Seeds a few agents.
* Used for unit tests.
* Client implementation uses interfaces only

* hubclient depends on Inventory and Transport. Allows plugging GitHub/S3/FS later.