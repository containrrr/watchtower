
# Resource limitation

On a host where many containers share the available memory it happens sometime that a misconfigured container can consume the total available memory, which will laid to an `out-of-memory` issue.
To avoid such situation you can activate the apply-resource-limit by setting it to true.

Watchtower will then additionally set the maximum of memory for each container to configured value.
## Settings

- `--apply-resource-limit` (env. `APPLY_RESOURCE_LIMIT`): 
Activate or deactivate the memory limitation. 
Default: `false` for deactivate.

- `--max-memory-per-container` (env. `MAX_MEMORY_PER_CONTAINER`):
This flag has an effect only if the `apply-resource-limit` is activate.
Value format is: plain number (integer) followed with unit `{G,g,M,m,K,k}` e.g 10M for 10 Megabyte.
Default: `4G`

Example:
- Activate the resource limitation and use the default max memory limit, which is 4G per container

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e APPLY_RESOURCE_LIMIT=true \
  containrrr/watchtower
```

- Apply resource limit and set max memory to 5g. Some possible values 512M, 1024K
```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  --apply-resource-limit=true \
  --max-memory-per-container 5g \
  containrrr/watchtower
```
``
