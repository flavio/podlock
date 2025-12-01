tilt_settings_file = "./tilt-settings.yaml"
settings = read_yaml(tilt_settings_file)

if settings.get("allow_default_k8s_context", False):
    allow_k8s_contexts('default')

update_settings(k8s_upsert_timeout_secs=300)

# Install cert-manager
#
# Note: We are not using the tilt cert-manager extension, since it creates a namespace to test cert-manager,
# which takes a long time to delete when running `tilt down`.
# We Install the cert-manager CRDs separately, so we are sure they will be avalable before the sbomscanner Helm chart is installed.
cert_manager_version = "v1.18.2"
local_resource(
    "cert-manager-crds",
    cmd="kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/{}/cert-manager.crds.yaml".format(
        cert_manager_version
    ),
)

load("ext://helm_resource", "helm_resource", "helm_repo")
helm_repo("jetstack", "https://charts.jetstack.io")
helm_resource(
    "cert-manager",
    "jetstack/cert-manager",
    namespace="cert-manager",
    flags=[
        "--version",
        cert_manager_version,
        "--create-namespace",
        "--set",
        "installCRDs=false",
    ],
    resource_deps=[
        "jetstack",
        "cert-manager-crds",
    ],
)

# Create the podlock namespace
# This is required since the helm() function doesn't support the create_namespace flag
load("ext://namespace", "namespace_create")
namespace_create("podlock")

registry = settings.get("registry")
controller_image = settings.get("controller").get("image")
nri_image = settings.get("nri").get("image")

yaml = helm(
    "./charts/podlock",
    name="podlock",
    namespace="podlock",
    set=[
        "global.cattle.systemDefaultRegistry=" + registry,
        "controller.image.repository=" + controller_image,
        "nri.image.repository=" + nri_image,
        "nri.logLevel=debug",
    ],
)

objects = decode_yaml_stream(yaml)
for o in objects:
    if o.get('kind') == 'Deployment':
        containers = o['spec']['template']['spec']['containers']
        # Remove securityContext to allow hot reloading
        for container in containers:
            if 'securityContext' in container:
                container['securityContext'] = {}
updated_yaml = encode_yaml_stream(objects)
k8s_yaml(updated_yaml)

# Hot reloading containers
local_resource(
    "controller_tilt",
    "make controller",
    deps=[
        "go.mod",
        "go.sum",
        "cmd/controller",
        "api",
        "internal/controller",
        "internal/webhook",
    ],
)

entrypoint = ["/controller"]
dockerfile = "./hack/Dockerfile.controller.tilt"

load("ext://restart_process", "docker_build_with_restart")
docker_build_with_restart(
    registry + "/" + controller_image,
    ".",
    dockerfile=dockerfile,
    entrypoint=entrypoint,
    # `only` here is important, otherwise, the container will get updated
    # on _any_ file change.
    only=[
        "./bin/controller",
    ],
    live_update=[
        sync("./bin/controller", "/controller"),
    ],
)

local_resource(
    "nri_tilt",
    "make nri",
    deps=[
        "go.mod",
        "go.sum",
        "cmd/nri",
        "cmd/seal",
        "cmd/swap-oci-hook",
        "api",
        "internal/cmdutil",
        "internal/nri",
        "pkg/constants",
    ],
)

local_resource(
    "seal_tilt",
    "make seal",
    deps=[
        "go.mod",
        "go.sum",
        "cmd/seal",
        "api",
        "internal/cmdutil",
        "internal/seal",
        "pkg/constants",
    ],
)

local_resource(
    "swap_oci_hook_tilt",
    "make swap-oci-hook",
    deps=[
        "go.mod",
        "go.sum",
        "cmd/swap-oci-hook",
        "internal/nri",
    ],
)

entrypoint = ["/nri"]
dockerfile = "./hack/Dockerfile.nri.tilt"

load("ext://restart_process", "docker_build_with_restart")
docker_build_with_restart(
    registry + "/" + nri_image,
    ".",
    dockerfile=dockerfile,
    entrypoint=entrypoint,
    # `only` here is important, otherwise, the container will get updated
    # on _any_ file change.
    only=[
        "./bin/nri",
        "./bin/seal",
        "./bin/swap-oci-hook",
    ],
    live_update=[
        sync("./bin/nri", "/nri"),
        sync("./bin/seal", "/seal"),
        sync("./bin/swap-oci-hook", "/swap-oci-hook"),
    ],
)
