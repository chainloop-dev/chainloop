/* eslint-disable */
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";
import {
  CraftingSchema,
  CraftingSchema_Material_MaterialType,
  craftingSchema_Material_MaterialTypeFromJSON,
  craftingSchema_Material_MaterialTypeToJSON,
  CraftingSchema_Runner_RunnerType,
  craftingSchema_Runner_RunnerTypeFromJSON,
  craftingSchema_Runner_RunnerTypeToJSON,
} from "../../workflowcontract/v1/crafting_schema";

export const protobufPackage = "attestation.v1";

export interface Attestation {
  initializedAt?: Date;
  finishedAt?: Date;
  workflow?: WorkflowMetadata;
  materials: { [key: string]: Attestation_Material };
  /** Annotations for the attestation */
  annotations: { [key: string]: string };
  /** List of env variables */
  envVars: { [key: string]: string };
  runnerUrl: string;
  runnerType: CraftingSchema_Runner_RunnerType;
  /** Head Commit of the environment where the attestation was executed (optional) */
  head?: Commit;
}

export interface Attestation_MaterialsEntry {
  key: string;
  value?: Attestation_Material;
}

export interface Attestation_AnnotationsEntry {
  key: string;
  value: string;
}

export interface Attestation_Material {
  string?: Attestation_Material_KeyVal | undefined;
  containerImage?: Attestation_Material_ContainerImage | undefined;
  artifact?: Attestation_Material_Artifact | undefined;
  addedAt?: Date;
  materialType: CraftingSchema_Material_MaterialType;
  /** Whether the material has been uploaded to the CAS */
  uploadedToCas: boolean;
  /**
   * If the material content has been injected inline in the attestation
   * leveraging a form of inline CAS
   */
  inlineCas: boolean;
  /** Annotations for the material */
  annotations: { [key: string]: string };
}

export interface Attestation_Material_AnnotationsEntry {
  key: string;
  value: string;
}

export interface Attestation_Material_KeyVal {
  id: string;
  value: string;
}

export interface Attestation_Material_ContainerImage {
  id: string;
  name: string;
  digest: string;
  isSubject: boolean;
  /** provided tag */
  tag: string;
}

export interface Attestation_Material_Artifact {
  /** ID of the artifact */
  id: string;
  /** filename, use for record purposes */
  name: string;
  /**
   * the digest is enough to retrieve the artifact since it's stored in a CAS
   * which also has annotated the fileName
   */
  digest: string;
  isSubject: boolean;
  /**
   * Inline content of the artifact.
   * This is optional and is used for small artifacts that can be stored inline in the attestation
   */
  content: Uint8Array;
}

export interface Attestation_EnvVarsEntry {
  key: string;
  value: string;
}

export interface Commit {
  hash: string;
  authorEmail: string;
  authorName: string;
  message: string;
  date?: Date;
  remotes: Commit_Remote[];
}

export interface Commit_Remote {
  name: string;
  url: string;
}

/** Intermediate information that will get stored in the system while the run is being executed */
export interface CraftingState {
  inputSchema?: CraftingSchema;
  attestation?: Attestation;
  dryRun: boolean;
}

export interface WorkflowMetadata {
  name: string;
  project: string;
  team: string;
  workflowId: string;
  /** Not required since we might be doing a dry-run */
  workflowRunId: string;
  schemaRevision: string;
  /** organization name */
  organization: string;
}

function createBaseAttestation(): Attestation {
  return {
    initializedAt: undefined,
    finishedAt: undefined,
    workflow: undefined,
    materials: {},
    annotations: {},
    envVars: {},
    runnerUrl: "",
    runnerType: 0,
    head: undefined,
  };
}

export const Attestation = {
  encode(message: Attestation, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.initializedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.initializedAt), writer.uint32(10).fork()).ldelim();
    }
    if (message.finishedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.finishedAt), writer.uint32(18).fork()).ldelim();
    }
    if (message.workflow !== undefined) {
      WorkflowMetadata.encode(message.workflow, writer.uint32(26).fork()).ldelim();
    }
    Object.entries(message.materials).forEach(([key, value]) => {
      Attestation_MaterialsEntry.encode({ key: key as any, value }, writer.uint32(34).fork()).ldelim();
    });
    Object.entries(message.annotations).forEach(([key, value]) => {
      Attestation_AnnotationsEntry.encode({ key: key as any, value }, writer.uint32(42).fork()).ldelim();
    });
    Object.entries(message.envVars).forEach(([key, value]) => {
      Attestation_EnvVarsEntry.encode({ key: key as any, value }, writer.uint32(50).fork()).ldelim();
    });
    if (message.runnerUrl !== "") {
      writer.uint32(58).string(message.runnerUrl);
    }
    if (message.runnerType !== 0) {
      writer.uint32(64).int32(message.runnerType);
    }
    if (message.head !== undefined) {
      Commit.encode(message.head, writer.uint32(74).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.initializedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.finishedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.workflow = WorkflowMetadata.decode(reader, reader.uint32());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          const entry4 = Attestation_MaterialsEntry.decode(reader, reader.uint32());
          if (entry4.value !== undefined) {
            message.materials[entry4.key] = entry4.value;
          }
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          const entry5 = Attestation_AnnotationsEntry.decode(reader, reader.uint32());
          if (entry5.value !== undefined) {
            message.annotations[entry5.key] = entry5.value;
          }
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          const entry6 = Attestation_EnvVarsEntry.decode(reader, reader.uint32());
          if (entry6.value !== undefined) {
            message.envVars[entry6.key] = entry6.value;
          }
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.runnerUrl = reader.string();
          continue;
        case 8:
          if (tag !== 64) {
            break;
          }

          message.runnerType = reader.int32() as any;
          continue;
        case 9:
          if (tag !== 74) {
            break;
          }

          message.head = Commit.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation {
    return {
      initializedAt: isSet(object.initializedAt) ? fromJsonTimestamp(object.initializedAt) : undefined,
      finishedAt: isSet(object.finishedAt) ? fromJsonTimestamp(object.finishedAt) : undefined,
      workflow: isSet(object.workflow) ? WorkflowMetadata.fromJSON(object.workflow) : undefined,
      materials: isObject(object.materials)
        ? Object.entries(object.materials).reduce<{ [key: string]: Attestation_Material }>((acc, [key, value]) => {
          acc[key] = Attestation_Material.fromJSON(value);
          return acc;
        }, {})
        : {},
      annotations: isObject(object.annotations)
        ? Object.entries(object.annotations).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      envVars: isObject(object.envVars)
        ? Object.entries(object.envVars).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
      runnerUrl: isSet(object.runnerUrl) ? String(object.runnerUrl) : "",
      runnerType: isSet(object.runnerType) ? craftingSchema_Runner_RunnerTypeFromJSON(object.runnerType) : 0,
      head: isSet(object.head) ? Commit.fromJSON(object.head) : undefined,
    };
  },

  toJSON(message: Attestation): unknown {
    const obj: any = {};
    message.initializedAt !== undefined && (obj.initializedAt = message.initializedAt.toISOString());
    message.finishedAt !== undefined && (obj.finishedAt = message.finishedAt.toISOString());
    message.workflow !== undefined &&
      (obj.workflow = message.workflow ? WorkflowMetadata.toJSON(message.workflow) : undefined);
    obj.materials = {};
    if (message.materials) {
      Object.entries(message.materials).forEach(([k, v]) => {
        obj.materials[k] = Attestation_Material.toJSON(v);
      });
    }
    obj.annotations = {};
    if (message.annotations) {
      Object.entries(message.annotations).forEach(([k, v]) => {
        obj.annotations[k] = v;
      });
    }
    obj.envVars = {};
    if (message.envVars) {
      Object.entries(message.envVars).forEach(([k, v]) => {
        obj.envVars[k] = v;
      });
    }
    message.runnerUrl !== undefined && (obj.runnerUrl = message.runnerUrl);
    message.runnerType !== undefined && (obj.runnerType = craftingSchema_Runner_RunnerTypeToJSON(message.runnerType));
    message.head !== undefined && (obj.head = message.head ? Commit.toJSON(message.head) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation>, I>>(base?: I): Attestation {
    return Attestation.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation>, I>>(object: I): Attestation {
    const message = createBaseAttestation();
    message.initializedAt = object.initializedAt ?? undefined;
    message.finishedAt = object.finishedAt ?? undefined;
    message.workflow = (object.workflow !== undefined && object.workflow !== null)
      ? WorkflowMetadata.fromPartial(object.workflow)
      : undefined;
    message.materials = Object.entries(object.materials ?? {}).reduce<{ [key: string]: Attestation_Material }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = Attestation_Material.fromPartial(value);
        }
        return acc;
      },
      {},
    );
    message.annotations = Object.entries(object.annotations ?? {}).reduce<{ [key: string]: string }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = String(value);
        }
        return acc;
      },
      {},
    );
    message.envVars = Object.entries(object.envVars ?? {}).reduce<{ [key: string]: string }>((acc, [key, value]) => {
      if (value !== undefined) {
        acc[key] = String(value);
      }
      return acc;
    }, {});
    message.runnerUrl = object.runnerUrl ?? "";
    message.runnerType = object.runnerType ?? 0;
    message.head = (object.head !== undefined && object.head !== null) ? Commit.fromPartial(object.head) : undefined;
    return message;
  },
};

function createBaseAttestation_MaterialsEntry(): Attestation_MaterialsEntry {
  return { key: "", value: undefined };
}

export const Attestation_MaterialsEntry = {
  encode(message: Attestation_MaterialsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== undefined) {
      Attestation_Material.encode(message.value, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_MaterialsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_MaterialsEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = Attestation_Material.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_MaterialsEntry {
    return {
      key: isSet(object.key) ? String(object.key) : "",
      value: isSet(object.value) ? Attestation_Material.fromJSON(object.value) : undefined,
    };
  },

  toJSON(message: Attestation_MaterialsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value ? Attestation_Material.toJSON(message.value) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_MaterialsEntry>, I>>(base?: I): Attestation_MaterialsEntry {
    return Attestation_MaterialsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_MaterialsEntry>, I>>(object: I): Attestation_MaterialsEntry {
    const message = createBaseAttestation_MaterialsEntry();
    message.key = object.key ?? "";
    message.value = (object.value !== undefined && object.value !== null)
      ? Attestation_Material.fromPartial(object.value)
      : undefined;
    return message;
  },
};

function createBaseAttestation_AnnotationsEntry(): Attestation_AnnotationsEntry {
  return { key: "", value: "" };
}

export const Attestation_AnnotationsEntry = {
  encode(message: Attestation_AnnotationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_AnnotationsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_AnnotationsEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_AnnotationsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: Attestation_AnnotationsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_AnnotationsEntry>, I>>(base?: I): Attestation_AnnotationsEntry {
    return Attestation_AnnotationsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_AnnotationsEntry>, I>>(object: I): Attestation_AnnotationsEntry {
    const message = createBaseAttestation_AnnotationsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseAttestation_Material(): Attestation_Material {
  return {
    string: undefined,
    containerImage: undefined,
    artifact: undefined,
    addedAt: undefined,
    materialType: 0,
    uploadedToCas: false,
    inlineCas: false,
    annotations: {},
  };
}

export const Attestation_Material = {
  encode(message: Attestation_Material, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.string !== undefined) {
      Attestation_Material_KeyVal.encode(message.string, writer.uint32(10).fork()).ldelim();
    }
    if (message.containerImage !== undefined) {
      Attestation_Material_ContainerImage.encode(message.containerImage, writer.uint32(18).fork()).ldelim();
    }
    if (message.artifact !== undefined) {
      Attestation_Material_Artifact.encode(message.artifact, writer.uint32(26).fork()).ldelim();
    }
    if (message.addedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.addedAt), writer.uint32(42).fork()).ldelim();
    }
    if (message.materialType !== 0) {
      writer.uint32(48).int32(message.materialType);
    }
    if (message.uploadedToCas === true) {
      writer.uint32(56).bool(message.uploadedToCas);
    }
    if (message.inlineCas === true) {
      writer.uint32(64).bool(message.inlineCas);
    }
    Object.entries(message.annotations).forEach(([key, value]) => {
      Attestation_Material_AnnotationsEntry.encode({ key: key as any, value }, writer.uint32(74).fork()).ldelim();
    });
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.string = Attestation_Material_KeyVal.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.containerImage = Attestation_Material_ContainerImage.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.artifact = Attestation_Material_Artifact.decode(reader, reader.uint32());
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.addedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 48) {
            break;
          }

          message.materialType = reader.int32() as any;
          continue;
        case 7:
          if (tag !== 56) {
            break;
          }

          message.uploadedToCas = reader.bool();
          continue;
        case 8:
          if (tag !== 64) {
            break;
          }

          message.inlineCas = reader.bool();
          continue;
        case 9:
          if (tag !== 74) {
            break;
          }

          const entry9 = Attestation_Material_AnnotationsEntry.decode(reader, reader.uint32());
          if (entry9.value !== undefined) {
            message.annotations[entry9.key] = entry9.value;
          }
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material {
    return {
      string: isSet(object.string) ? Attestation_Material_KeyVal.fromJSON(object.string) : undefined,
      containerImage: isSet(object.containerImage)
        ? Attestation_Material_ContainerImage.fromJSON(object.containerImage)
        : undefined,
      artifact: isSet(object.artifact) ? Attestation_Material_Artifact.fromJSON(object.artifact) : undefined,
      addedAt: isSet(object.addedAt) ? fromJsonTimestamp(object.addedAt) : undefined,
      materialType: isSet(object.materialType) ? craftingSchema_Material_MaterialTypeFromJSON(object.materialType) : 0,
      uploadedToCas: isSet(object.uploadedToCas) ? Boolean(object.uploadedToCas) : false,
      inlineCas: isSet(object.inlineCas) ? Boolean(object.inlineCas) : false,
      annotations: isObject(object.annotations)
        ? Object.entries(object.annotations).reduce<{ [key: string]: string }>((acc, [key, value]) => {
          acc[key] = String(value);
          return acc;
        }, {})
        : {},
    };
  },

  toJSON(message: Attestation_Material): unknown {
    const obj: any = {};
    message.string !== undefined &&
      (obj.string = message.string ? Attestation_Material_KeyVal.toJSON(message.string) : undefined);
    message.containerImage !== undefined && (obj.containerImage = message.containerImage
      ? Attestation_Material_ContainerImage.toJSON(message.containerImage)
      : undefined);
    message.artifact !== undefined &&
      (obj.artifact = message.artifact ? Attestation_Material_Artifact.toJSON(message.artifact) : undefined);
    message.addedAt !== undefined && (obj.addedAt = message.addedAt.toISOString());
    message.materialType !== undefined &&
      (obj.materialType = craftingSchema_Material_MaterialTypeToJSON(message.materialType));
    message.uploadedToCas !== undefined && (obj.uploadedToCas = message.uploadedToCas);
    message.inlineCas !== undefined && (obj.inlineCas = message.inlineCas);
    obj.annotations = {};
    if (message.annotations) {
      Object.entries(message.annotations).forEach(([k, v]) => {
        obj.annotations[k] = v;
      });
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material>, I>>(base?: I): Attestation_Material {
    return Attestation_Material.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material>, I>>(object: I): Attestation_Material {
    const message = createBaseAttestation_Material();
    message.string = (object.string !== undefined && object.string !== null)
      ? Attestation_Material_KeyVal.fromPartial(object.string)
      : undefined;
    message.containerImage = (object.containerImage !== undefined && object.containerImage !== null)
      ? Attestation_Material_ContainerImage.fromPartial(object.containerImage)
      : undefined;
    message.artifact = (object.artifact !== undefined && object.artifact !== null)
      ? Attestation_Material_Artifact.fromPartial(object.artifact)
      : undefined;
    message.addedAt = object.addedAt ?? undefined;
    message.materialType = object.materialType ?? 0;
    message.uploadedToCas = object.uploadedToCas ?? false;
    message.inlineCas = object.inlineCas ?? false;
    message.annotations = Object.entries(object.annotations ?? {}).reduce<{ [key: string]: string }>(
      (acc, [key, value]) => {
        if (value !== undefined) {
          acc[key] = String(value);
        }
        return acc;
      },
      {},
    );
    return message;
  },
};

function createBaseAttestation_Material_AnnotationsEntry(): Attestation_Material_AnnotationsEntry {
  return { key: "", value: "" };
}

export const Attestation_Material_AnnotationsEntry = {
  encode(message: Attestation_Material_AnnotationsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material_AnnotationsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material_AnnotationsEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material_AnnotationsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: Attestation_Material_AnnotationsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material_AnnotationsEntry>, I>>(
    base?: I,
  ): Attestation_Material_AnnotationsEntry {
    return Attestation_Material_AnnotationsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material_AnnotationsEntry>, I>>(
    object: I,
  ): Attestation_Material_AnnotationsEntry {
    const message = createBaseAttestation_Material_AnnotationsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseAttestation_Material_KeyVal(): Attestation_Material_KeyVal {
  return { id: "", value: "" };
}

export const Attestation_Material_KeyVal = {
  encode(message: Attestation_Material_KeyVal, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material_KeyVal {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material_KeyVal();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material_KeyVal {
    return { id: isSet(object.id) ? String(object.id) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: Attestation_Material_KeyVal): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material_KeyVal>, I>>(base?: I): Attestation_Material_KeyVal {
    return Attestation_Material_KeyVal.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material_KeyVal>, I>>(object: I): Attestation_Material_KeyVal {
    const message = createBaseAttestation_Material_KeyVal();
    message.id = object.id ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseAttestation_Material_ContainerImage(): Attestation_Material_ContainerImage {
  return { id: "", name: "", digest: "", isSubject: false, tag: "" };
}

export const Attestation_Material_ContainerImage = {
  encode(message: Attestation_Material_ContainerImage, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.digest !== "") {
      writer.uint32(26).string(message.digest);
    }
    if (message.isSubject === true) {
      writer.uint32(32).bool(message.isSubject);
    }
    if (message.tag !== "") {
      writer.uint32(42).string(message.tag);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material_ContainerImage {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material_ContainerImage();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.name = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.digest = reader.string();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.isSubject = reader.bool();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.tag = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material_ContainerImage {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      digest: isSet(object.digest) ? String(object.digest) : "",
      isSubject: isSet(object.isSubject) ? Boolean(object.isSubject) : false,
      tag: isSet(object.tag) ? String(object.tag) : "",
    };
  },

  toJSON(message: Attestation_Material_ContainerImage): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.digest !== undefined && (obj.digest = message.digest);
    message.isSubject !== undefined && (obj.isSubject = message.isSubject);
    message.tag !== undefined && (obj.tag = message.tag);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material_ContainerImage>, I>>(
    base?: I,
  ): Attestation_Material_ContainerImage {
    return Attestation_Material_ContainerImage.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material_ContainerImage>, I>>(
    object: I,
  ): Attestation_Material_ContainerImage {
    const message = createBaseAttestation_Material_ContainerImage();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.digest = object.digest ?? "";
    message.isSubject = object.isSubject ?? false;
    message.tag = object.tag ?? "";
    return message;
  },
};

function createBaseAttestation_Material_Artifact(): Attestation_Material_Artifact {
  return { id: "", name: "", digest: "", isSubject: false, content: new Uint8Array(0) };
}

export const Attestation_Material_Artifact = {
  encode(message: Attestation_Material_Artifact, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.digest !== "") {
      writer.uint32(26).string(message.digest);
    }
    if (message.isSubject === true) {
      writer.uint32(32).bool(message.isSubject);
    }
    if (message.content.length !== 0) {
      writer.uint32(42).bytes(message.content);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_Material_Artifact {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_Material_Artifact();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.name = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.digest = reader.string();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.isSubject = reader.bool();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.content = reader.bytes();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_Material_Artifact {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      digest: isSet(object.digest) ? String(object.digest) : "",
      isSubject: isSet(object.isSubject) ? Boolean(object.isSubject) : false,
      content: isSet(object.content) ? bytesFromBase64(object.content) : new Uint8Array(0),
    };
  },

  toJSON(message: Attestation_Material_Artifact): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.digest !== undefined && (obj.digest = message.digest);
    message.isSubject !== undefined && (obj.isSubject = message.isSubject);
    message.content !== undefined &&
      (obj.content = base64FromBytes(message.content !== undefined ? message.content : new Uint8Array(0)));
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_Material_Artifact>, I>>(base?: I): Attestation_Material_Artifact {
    return Attestation_Material_Artifact.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_Material_Artifact>, I>>(
    object: I,
  ): Attestation_Material_Artifact {
    const message = createBaseAttestation_Material_Artifact();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.digest = object.digest ?? "";
    message.isSubject = object.isSubject ?? false;
    message.content = object.content ?? new Uint8Array(0);
    return message;
  },
};

function createBaseAttestation_EnvVarsEntry(): Attestation_EnvVarsEntry {
  return { key: "", value: "" };
}

export const Attestation_EnvVarsEntry = {
  encode(message: Attestation_EnvVarsEntry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.key !== "") {
      writer.uint32(10).string(message.key);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Attestation_EnvVarsEntry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAttestation_EnvVarsEntry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.key = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.value = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Attestation_EnvVarsEntry {
    return { key: isSet(object.key) ? String(object.key) : "", value: isSet(object.value) ? String(object.value) : "" };
  },

  toJSON(message: Attestation_EnvVarsEntry): unknown {
    const obj: any = {};
    message.key !== undefined && (obj.key = message.key);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<Attestation_EnvVarsEntry>, I>>(base?: I): Attestation_EnvVarsEntry {
    return Attestation_EnvVarsEntry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Attestation_EnvVarsEntry>, I>>(object: I): Attestation_EnvVarsEntry {
    const message = createBaseAttestation_EnvVarsEntry();
    message.key = object.key ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

function createBaseCommit(): Commit {
  return { hash: "", authorEmail: "", authorName: "", message: "", date: undefined, remotes: [] };
}

export const Commit = {
  encode(message: Commit, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.hash !== "") {
      writer.uint32(10).string(message.hash);
    }
    if (message.authorEmail !== "") {
      writer.uint32(18).string(message.authorEmail);
    }
    if (message.authorName !== "") {
      writer.uint32(26).string(message.authorName);
    }
    if (message.message !== "") {
      writer.uint32(34).string(message.message);
    }
    if (message.date !== undefined) {
      Timestamp.encode(toTimestamp(message.date), writer.uint32(42).fork()).ldelim();
    }
    for (const v of message.remotes) {
      Commit_Remote.encode(v!, writer.uint32(50).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Commit {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCommit();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.hash = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.authorEmail = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.authorName = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.message = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.date = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.remotes.push(Commit_Remote.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Commit {
    return {
      hash: isSet(object.hash) ? String(object.hash) : "",
      authorEmail: isSet(object.authorEmail) ? String(object.authorEmail) : "",
      authorName: isSet(object.authorName) ? String(object.authorName) : "",
      message: isSet(object.message) ? String(object.message) : "",
      date: isSet(object.date) ? fromJsonTimestamp(object.date) : undefined,
      remotes: Array.isArray(object?.remotes) ? object.remotes.map((e: any) => Commit_Remote.fromJSON(e)) : [],
    };
  },

  toJSON(message: Commit): unknown {
    const obj: any = {};
    message.hash !== undefined && (obj.hash = message.hash);
    message.authorEmail !== undefined && (obj.authorEmail = message.authorEmail);
    message.authorName !== undefined && (obj.authorName = message.authorName);
    message.message !== undefined && (obj.message = message.message);
    message.date !== undefined && (obj.date = message.date.toISOString());
    if (message.remotes) {
      obj.remotes = message.remotes.map((e) => e ? Commit_Remote.toJSON(e) : undefined);
    } else {
      obj.remotes = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<Commit>, I>>(base?: I): Commit {
    return Commit.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Commit>, I>>(object: I): Commit {
    const message = createBaseCommit();
    message.hash = object.hash ?? "";
    message.authorEmail = object.authorEmail ?? "";
    message.authorName = object.authorName ?? "";
    message.message = object.message ?? "";
    message.date = object.date ?? undefined;
    message.remotes = object.remotes?.map((e) => Commit_Remote.fromPartial(e)) || [];
    return message;
  },
};

function createBaseCommit_Remote(): Commit_Remote {
  return { name: "", url: "" };
}

export const Commit_Remote = {
  encode(message: Commit_Remote, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.url !== "") {
      writer.uint32(18).string(message.url);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Commit_Remote {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCommit_Remote();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.name = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.url = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Commit_Remote {
    return { name: isSet(object.name) ? String(object.name) : "", url: isSet(object.url) ? String(object.url) : "" };
  },

  toJSON(message: Commit_Remote): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.url !== undefined && (obj.url = message.url);
    return obj;
  },

  create<I extends Exact<DeepPartial<Commit_Remote>, I>>(base?: I): Commit_Remote {
    return Commit_Remote.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Commit_Remote>, I>>(object: I): Commit_Remote {
    const message = createBaseCommit_Remote();
    message.name = object.name ?? "";
    message.url = object.url ?? "";
    return message;
  },
};

function createBaseCraftingState(): CraftingState {
  return { inputSchema: undefined, attestation: undefined, dryRun: false };
}

export const CraftingState = {
  encode(message: CraftingState, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.inputSchema !== undefined) {
      CraftingSchema.encode(message.inputSchema, writer.uint32(10).fork()).ldelim();
    }
    if (message.attestation !== undefined) {
      Attestation.encode(message.attestation, writer.uint32(18).fork()).ldelim();
    }
    if (message.dryRun === true) {
      writer.uint32(24).bool(message.dryRun);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CraftingState {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCraftingState();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.inputSchema = CraftingSchema.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.attestation = Attestation.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.dryRun = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CraftingState {
    return {
      inputSchema: isSet(object.inputSchema) ? CraftingSchema.fromJSON(object.inputSchema) : undefined,
      attestation: isSet(object.attestation) ? Attestation.fromJSON(object.attestation) : undefined,
      dryRun: isSet(object.dryRun) ? Boolean(object.dryRun) : false,
    };
  },

  toJSON(message: CraftingState): unknown {
    const obj: any = {};
    message.inputSchema !== undefined &&
      (obj.inputSchema = message.inputSchema ? CraftingSchema.toJSON(message.inputSchema) : undefined);
    message.attestation !== undefined &&
      (obj.attestation = message.attestation ? Attestation.toJSON(message.attestation) : undefined);
    message.dryRun !== undefined && (obj.dryRun = message.dryRun);
    return obj;
  },

  create<I extends Exact<DeepPartial<CraftingState>, I>>(base?: I): CraftingState {
    return CraftingState.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CraftingState>, I>>(object: I): CraftingState {
    const message = createBaseCraftingState();
    message.inputSchema = (object.inputSchema !== undefined && object.inputSchema !== null)
      ? CraftingSchema.fromPartial(object.inputSchema)
      : undefined;
    message.attestation = (object.attestation !== undefined && object.attestation !== null)
      ? Attestation.fromPartial(object.attestation)
      : undefined;
    message.dryRun = object.dryRun ?? false;
    return message;
  },
};

function createBaseWorkflowMetadata(): WorkflowMetadata {
  return { name: "", project: "", team: "", workflowId: "", workflowRunId: "", schemaRevision: "", organization: "" };
}

export const WorkflowMetadata = {
  encode(message: WorkflowMetadata, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.project !== "") {
      writer.uint32(18).string(message.project);
    }
    if (message.team !== "") {
      writer.uint32(26).string(message.team);
    }
    if (message.workflowId !== "") {
      writer.uint32(42).string(message.workflowId);
    }
    if (message.workflowRunId !== "") {
      writer.uint32(50).string(message.workflowRunId);
    }
    if (message.schemaRevision !== "") {
      writer.uint32(58).string(message.schemaRevision);
    }
    if (message.organization !== "") {
      writer.uint32(66).string(message.organization);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): WorkflowMetadata {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseWorkflowMetadata();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.name = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.project = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.team = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.workflowId = reader.string();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.workflowRunId = reader.string();
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.schemaRevision = reader.string();
          continue;
        case 8:
          if (tag !== 66) {
            break;
          }

          message.organization = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): WorkflowMetadata {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      project: isSet(object.project) ? String(object.project) : "",
      team: isSet(object.team) ? String(object.team) : "",
      workflowId: isSet(object.workflowId) ? String(object.workflowId) : "",
      workflowRunId: isSet(object.workflowRunId) ? String(object.workflowRunId) : "",
      schemaRevision: isSet(object.schemaRevision) ? String(object.schemaRevision) : "",
      organization: isSet(object.organization) ? String(object.organization) : "",
    };
  },

  toJSON(message: WorkflowMetadata): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.project !== undefined && (obj.project = message.project);
    message.team !== undefined && (obj.team = message.team);
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    message.workflowRunId !== undefined && (obj.workflowRunId = message.workflowRunId);
    message.schemaRevision !== undefined && (obj.schemaRevision = message.schemaRevision);
    message.organization !== undefined && (obj.organization = message.organization);
    return obj;
  },

  create<I extends Exact<DeepPartial<WorkflowMetadata>, I>>(base?: I): WorkflowMetadata {
    return WorkflowMetadata.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<WorkflowMetadata>, I>>(object: I): WorkflowMetadata {
    const message = createBaseWorkflowMetadata();
    message.name = object.name ?? "";
    message.project = object.project ?? "";
    message.team = object.team ?? "";
    message.workflowId = object.workflowId ?? "";
    message.workflowRunId = object.workflowRunId ?? "";
    message.schemaRevision = object.schemaRevision ?? "";
    message.organization = object.organization ?? "";
    return message;
  },
};

declare var self: any | undefined;
declare var window: any | undefined;
declare var global: any | undefined;
var tsProtoGlobalThis: any = (() => {
  if (typeof globalThis !== "undefined") {
    return globalThis;
  }
  if (typeof self !== "undefined") {
    return self;
  }
  if (typeof window !== "undefined") {
    return window;
  }
  if (typeof global !== "undefined") {
    return global;
  }
  throw "Unable to locate global object";
})();

function bytesFromBase64(b64: string): Uint8Array {
  if (tsProtoGlobalThis.Buffer) {
    return Uint8Array.from(tsProtoGlobalThis.Buffer.from(b64, "base64"));
  } else {
    const bin = tsProtoGlobalThis.atob(b64);
    const arr = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; ++i) {
      arr[i] = bin.charCodeAt(i);
    }
    return arr;
  }
}

function base64FromBytes(arr: Uint8Array): string {
  if (tsProtoGlobalThis.Buffer) {
    return tsProtoGlobalThis.Buffer.from(arr).toString("base64");
  } else {
    const bin: string[] = [];
    arr.forEach((byte) => {
      bin.push(String.fromCharCode(byte));
    });
    return tsProtoGlobalThis.btoa(bin.join(""));
  }
}

type Builtin = Date | Function | Uint8Array | string | number | boolean | undefined;

export type DeepPartial<T> = T extends Builtin ? T
  : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>>
  : T extends {} ? { [K in keyof T]?: DeepPartial<T[K]> }
  : Partial<T>;

type KeysOfUnion<T> = T extends T ? keyof T : never;
export type Exact<P, I extends P> = P extends Builtin ? P
  : P & { [K in keyof P]: Exact<P[K], I[K]> } & { [K in Exclude<keyof I, KeysOfUnion<P>>]: never };

function toTimestamp(date: Date): Timestamp {
  const seconds = date.getTime() / 1_000;
  const nanos = (date.getTime() % 1_000) * 1_000_000;
  return { seconds, nanos };
}

function fromTimestamp(t: Timestamp): Date {
  let millis = (t.seconds || 0) * 1_000;
  millis += (t.nanos || 0) / 1_000_000;
  return new Date(millis);
}

function fromJsonTimestamp(o: any): Date {
  if (o instanceof Date) {
    return o;
  } else if (typeof o === "string") {
    return new Date(o);
  } else {
    return fromTimestamp(Timestamp.fromJSON(o));
  }
}

function isObject(value: any): boolean {
  return typeof value === "object" && value !== null;
}

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}
