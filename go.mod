module github.com/chainloop-dev/chainloop

go 1.23.4

require (
	cloud.google.com/go/secretmanager v1.14.0
	code.cloudfoundry.org/bytefmt v0.0.0-20230612151507-41ef4d1f67a4
	cuelang.org/go v0.9.2
	entgo.io/ent v0.14.1
	github.com/adrg/xdg v0.4.0
	github.com/aws/aws-sdk-go-v2 v1.30.5
	github.com/aws/aws-sdk-go-v2/config v1.27.33
	github.com/aws/aws-sdk-go-v2/credentials v1.17.32
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.28.6
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.7
	github.com/aws/smithy-go v1.20.4
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/coreos/go-oidc/v3 v3.11.0
	github.com/docker/distribution v2.8.3+incompatible
	github.com/docker/go-connections v0.5.0
	github.com/getsentry/sentry-go v0.23.0
	github.com/go-kratos/kratos/contrib/log/zap/v2 v2.0.0-20230823024326-a09f4d8ebba9
	github.com/go-kratos/kratos/v2 v2.7.0
	github.com/golang-jwt/jwt/v4 v4.5.1
	github.com/google/go-containerregistry v0.20.2
	github.com/google/subcommands v1.2.0
	github.com/google/uuid v1.6.0
	github.com/google/wire v0.6.0
	github.com/googleapis/gax-go/v2 v2.13.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.1-0.20210315223345-82c243799c99
	github.com/hashicorp/vault/api v1.14.0
	github.com/hedwigz/entviz v0.0.0-20221011080911-9d47f6f1d818
	github.com/improbable-eng/grpc-web v0.15.0
	github.com/in-toto/in-toto-golang v0.9.0 // indirect
	github.com/jedib0t/go-pretty/v6 v6.4.7
	github.com/joshdk/go-junit v1.0.0
	github.com/lib/pq v1.10.9
	github.com/moby/moby v26.1.0+incompatible
	github.com/opencontainers/image-spec v1.1.0
	github.com/prometheus/client_golang v1.20.2
	github.com/rs/zerolog v1.32.0
	github.com/secure-systems-lab/go-securesystemslib v0.8.0
	github.com/sigstore/cosign/v2 v2.4.1
	github.com/sigstore/sigstore v1.8.9
	github.com/spdx/tools-golang v0.5.3
	github.com/spf13/cobra v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.19.0
	github.com/stretchr/testify v1.10.0
	github.com/testcontainers/testcontainers-go v0.32.0
	go.uber.org/automaxprocs v1.5.3
	go.uber.org/zap v1.27.0
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56
	golang.org/x/oauth2 v0.23.0
	golang.org/x/term v0.28.0
	google.golang.org/api v0.196.0
	google.golang.org/genproto v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/grpc v1.66.0
	google.golang.org/protobuf v1.36.1
	sigs.k8s.io/yaml v1.4.0
)

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.33.0-20240401165935-b983156c5e99.1
	cloud.google.com/go/storage v1.43.0
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.14.0
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.7.0
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.3.1
	github.com/bufbuild/protovalidate-go v0.6.1
	github.com/bufbuild/protoyaml-go v0.1.11
	github.com/casbin/casbin/v2 v2.101.0
	github.com/casbin/ent-adapter v0.4.0
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/google/go-github/v66 v66.0.0
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.1.0
	github.com/in-toto/attestation v1.1.0
	github.com/invopop/jsonschema v0.7.0
	github.com/jackc/pgx/v5 v5.7.1
	github.com/muesli/reflow v0.3.0
	github.com/nats-io/nats.go v1.34.0
	github.com/open-policy-agent/opa v0.68.0
	github.com/openvex/go-vex v0.2.5
	github.com/posthog/posthog-go v0.0.0-20240327112532-87b23fe11103
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1
	github.com/sigstore/fulcio v1.6.3
	github.com/sigstore/protobuf-specs v0.3.2
	github.com/sigstore/sigstore/pkg/signature/kms/aws v1.8.8
	github.com/sigstore/sigstore/pkg/signature/kms/azure v1.8.8
	github.com/sigstore/sigstore/pkg/signature/kms/gcp v1.8.8
	github.com/sigstore/sigstore/pkg/signature/kms/hashivault v1.8.8
	gitlab.com/gitlab-org/security-products/analyzers/report/v5 v5.3.0
	google.golang.org/genproto/googleapis/api v0.0.0-20240827150818-7e3bb234dfed
	google.golang.org/genproto/googleapis/bytestream v0.0.0-20240903143218-8af14fe29dc1
)

require (
	cloud.google.com/go/auth v0.9.3 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.4 // indirect
	cloud.google.com/go/kms v1.19.0 // indirect
	cloud.google.com/go/longrunning v0.6.0 // indirect
	cloud.google.com/go/pubsub v1.42.0 // indirect
	dario.cat/mergo v1.0.0 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.10.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/keyvault/internal v0.7.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys v1.1.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/internal v1.0.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/Microsoft/hcsshim v0.11.7 // indirect
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/agnivade/levenshtein v1.1.1 // indirect
	github.com/anchore/go-struct-converter v0.0.0-20230627203149-c72ef8859ca9 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/kms v1.35.7 // indirect
	github.com/bmatcuk/doublestar v1.3.4 // indirect
	github.com/bmatcuk/doublestar/v4 v4.7.1 // indirect
	github.com/casbin/govaluate v1.2.0 // indirect
	github.com/cockroachdb/apd/v3 v3.2.1 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/cpuguy83/dockercfg v0.3.1 // indirect
	github.com/creack/pty v1.1.21 // indirect
	github.com/cyphar/filepath-securejoin v0.3.6 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.1 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-jose/go-jose/v4 v4.0.4 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-playground/assert/v2 v2.2.0 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/goadesign/goa v2.2.5+incompatible // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/google/cel-go v0.20.1 // indirect
	github.com/google/gnostic-models v0.6.9-0.20230804172637-c7be7c783f49 // indirect
	github.com/google/go-github/v55 v55.0.0 // indirect
	github.com/google/renameio/v2 v2.0.0 // indirect
	github.com/gorilla/handlers v1.5.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jellydator/ttlcache/v3 v3.2.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20230326075908-cb1d2100619a // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/user v0.3.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/nozzle/throttler v0.0.0-20180817012639-2ea982251481 // indirect
	github.com/oklog/run v1.1.0 // indirect
	github.com/onsi/ginkgo/v2 v2.19.0 // indirect
	github.com/otiai10/copy v1.11.0 // indirect
	github.com/package-url/packageurl-go v0.1.1 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/xattr v0.4.9 // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/shirou/gopsutil/v3 v3.23.12 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/shoenig/test v0.6.6 // indirect
	github.com/skeema/knownhosts v1.3.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spiffe/go-spiffe/v2 v2.3.0 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/tchap/go-patricia/v2 v2.3.1 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/yashtewari/glob-intersection v0.2.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	gitlab.com/gitlab-org/security-products/analyzers/common/v3 v3.2.1 // indirect
	gitlab.com/gitlab-org/security-products/analyzers/ruleset/v3 v3.0.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.54.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.54.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.22.0 // indirect
	go.opentelemetry.io/otel/metric v1.29.0 // indirect
	go.opentelemetry.io/otel/sdk v1.29.0 // indirect
	go.step.sm/crypto v0.51.2 // indirect
	goa.design/goa v2.2.5+incompatible // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

require (
	ariga.io/atlas v0.28.1 // indirect
	cloud.google.com/go v0.115.1 // indirect
	cloud.google.com/go/compute/metadata v0.5.0 // indirect
	cloud.google.com/go/iam v1.2.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets v0.12.0
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/IguteChung/casbin-psql-watcher v1.0.0
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.1.3 // indirect
	github.com/ThalesIgnite/crypto11 v1.2.5 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-sdk-go v1.55.5
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.7 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cenkalti/backoff/v3 v3.2.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/containerd/containerd v1.7.21 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20231011164504-785e29786b46 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/digitorus/pkcs7 v0.0.0-20230818184609-3a137a874352 // indirect
	github.com/digitorus/timestamp v0.0.0-20231217203849-220c5c2851b7 // indirect
	github.com/docker/cli v27.1.1+incompatible // indirect
	github.com/docker/docker v27.1.1+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fsouza/fake-gcs-server v1.47.6
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-git/go-git/v5 v5.13.1
	github.com/go-jose/go-jose/v3 v3.0.3 // indirect
	github.com/go-kratos/aegis v0.2.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v0.23.0 // indirect
	github.com/go-openapi/errors v0.22.0 // indirect
	github.com/go-openapi/inflect v0.21.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/loads v0.22.0 // indirect
	github.com/go-openapi/runtime v0.28.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/strfmt v0.23.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-openapi/validate v0.24.0 // indirect
	github.com/go-playground/form/v4 v4.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/certificate-transparency-go v1.2.1 // indirect
	github.com/google/go-cmp v0.6.0
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.3 // indirect
	github.com/gorilla/mux v1.8.1
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-plugin v1.6.3
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.1.7 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.5 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-5 // indirect
	github.com/hashicorp/hcl/v2 v2.23.0 // indirect
	github.com/iancoleman/orderedmap v0.3.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jedisct1/go-minisign v0.0.0-20230811132847-661be99b8267 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/letsencrypt/boulder v0.0.0-20240620165639-de9c06129bec // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/minio/minio-go/v7 v7.0.63
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/owenrumney/go-sarif v1.1.1
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/rs/cors v1.11.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/sassoftware/relic v7.2.1+incompatible // indirect
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/sigstore/rekor v1.3.6 // indirect
	github.com/sigstore/timestamp-authority v1.2.2 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/thales-e-security/pool v0.0.2 // indirect
	github.com/theupdateframework/go-tuf v0.7.0 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/transparency-dev/merkle v0.0.2 // indirect
	github.com/vbatts/tar-split v0.11.5 // indirect
	github.com/xanzy/go-gitlab v0.109.0 // indirect
	github.com/zclconf/go-cty v1.15.0 // indirect
	go.mongodb.org/mongo-driver v1.14.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel v1.29.0 // indirect
	go.opentelemetry.io/otel/trace v1.29.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.32.0
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.6.0 // indirect
	golang.org/x/tools v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.28.6 // indirect
	k8s.io/apimachinery v0.28.6
	k8s.io/client-go v0.28.6 // indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-openapi v0.0.0-20231010175941-2dd684a91f00 // indirect
	k8s.io/utils v0.0.0-20240502163921-fe8a2dddb1d0 // indirect
	nhooyr.io/websocket v1.8.10 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
)
