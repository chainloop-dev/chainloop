/* eslint-disable */
import _m0 from "protobufjs/minimal";

export const protobufPackage = "workflowcontract.v1";

/**
 * Schema definition provided by the user to the tool
 * that defines the schema of the workflowRun
 */
export interface CraftingSchema {
  /** Version of the schema, do not confuse with the revision of the content */
  schemaVersion: string;
  materials: CraftingSchema_Material[];
  envAllowList: string[];
  runner?: CraftingSchema_Runner;
  /**
   * List of annotations that can be used to add metadata to the attestation
   * this metadata can be used later on by the integrations engine to filter and interpolate data
   * It works in addition to the annotations defined in the materials and the runner
   */
  annotations: Annotation[];
}

export interface CraftingSchema_Runner {
  type: CraftingSchema_Runner_RunnerType;
}

export enum CraftingSchema_Runner_RunnerType {
  RUNNER_TYPE_UNSPECIFIED = 0,
  GITHUB_ACTION = 1,
  GITLAB_PIPELINE = 2,
  AZURE_PIPELINE = 3,
  JENKINS_JOB = 4,
  CIRCLECI_BUILD = 5,
  DAGGER_PIPELINE = 6,
  UNRECOGNIZED = -1,
}

export function craftingSchema_Runner_RunnerTypeFromJSON(object: any): CraftingSchema_Runner_RunnerType {
  switch (object) {
    case 0:
    case "RUNNER_TYPE_UNSPECIFIED":
      return CraftingSchema_Runner_RunnerType.RUNNER_TYPE_UNSPECIFIED;
    case 1:
    case "GITHUB_ACTION":
      return CraftingSchema_Runner_RunnerType.GITHUB_ACTION;
    case 2:
    case "GITLAB_PIPELINE":
      return CraftingSchema_Runner_RunnerType.GITLAB_PIPELINE;
    case 3:
    case "AZURE_PIPELINE":
      return CraftingSchema_Runner_RunnerType.AZURE_PIPELINE;
    case 4:
    case "JENKINS_JOB":
      return CraftingSchema_Runner_RunnerType.JENKINS_JOB;
    case 5:
    case "CIRCLECI_BUILD":
      return CraftingSchema_Runner_RunnerType.CIRCLECI_BUILD;
    case 6:
    case "DAGGER_PIPELINE":
      return CraftingSchema_Runner_RunnerType.DAGGER_PIPELINE;
    case -1:
    case "UNRECOGNIZED":
    default:
      return CraftingSchema_Runner_RunnerType.UNRECOGNIZED;
  }
}

export function craftingSchema_Runner_RunnerTypeToJSON(object: CraftingSchema_Runner_RunnerType): string {
  switch (object) {
    case CraftingSchema_Runner_RunnerType.RUNNER_TYPE_UNSPECIFIED:
      return "RUNNER_TYPE_UNSPECIFIED";
    case CraftingSchema_Runner_RunnerType.GITHUB_ACTION:
      return "GITHUB_ACTION";
    case CraftingSchema_Runner_RunnerType.GITLAB_PIPELINE:
      return "GITLAB_PIPELINE";
    case CraftingSchema_Runner_RunnerType.AZURE_PIPELINE:
      return "AZURE_PIPELINE";
    case CraftingSchema_Runner_RunnerType.JENKINS_JOB:
      return "JENKINS_JOB";
    case CraftingSchema_Runner_RunnerType.CIRCLECI_BUILD:
      return "CIRCLECI_BUILD";
    case CraftingSchema_Runner_RunnerType.DAGGER_PIPELINE:
      return "DAGGER_PIPELINE";
    case CraftingSchema_Runner_RunnerType.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface CraftingSchema_Material {
  type: CraftingSchema_Material_MaterialType;
  /** Single word optionally separated with _ or - */
  name: string;
  optional: boolean;
  /** If a material is set as output it will get added to the subject in the statement */
  output: boolean;
  /**
   * List of annotations that can be used to add metadata to the material
   * this metadata can be used later on by the integrations engine to filter and interpolate data
   */
  annotations: Annotation[];
}

export enum CraftingSchema_Material_MaterialType {
  MATERIAL_TYPE_UNSPECIFIED = 0,
  STRING = 1,
  CONTAINER_IMAGE = 2,
  ARTIFACT = 3,
  SBOM_CYCLONEDX_JSON = 4,
  SBOM_SPDX_JSON = 5,
  JUNIT_XML = 6,
  /** OPENVEX - https://github.com/openvex/spec */
  OPENVEX = 7,
  /** CSAF_VEX - https://docs.oasis-open.org/csaf/csaf/v2.0/cs03/csaf-v2.0-cs03.html */
  CSAF_VEX = 8,
  /**
   * SARIF - Static analysis output format
   * https://github.com/microsoft/sarif-tutorials/blob/main/docs/1-Introduction.md
   */
  SARIF = 9,
  HELM_CHART = 10,
  /**
   * EVIDENCE - Pieces of evidences represent generic, additional context that don't fit
   * into one of the well known material types. For example, a custom approval report (in json), ...
   */
  EVIDENCE = 11,
  UNRECOGNIZED = -1,
}

export function craftingSchema_Material_MaterialTypeFromJSON(object: any): CraftingSchema_Material_MaterialType {
  switch (object) {
    case 0:
    case "MATERIAL_TYPE_UNSPECIFIED":
      return CraftingSchema_Material_MaterialType.MATERIAL_TYPE_UNSPECIFIED;
    case 1:
    case "STRING":
      return CraftingSchema_Material_MaterialType.STRING;
    case 2:
    case "CONTAINER_IMAGE":
      return CraftingSchema_Material_MaterialType.CONTAINER_IMAGE;
    case 3:
    case "ARTIFACT":
      return CraftingSchema_Material_MaterialType.ARTIFACT;
    case 4:
    case "SBOM_CYCLONEDX_JSON":
      return CraftingSchema_Material_MaterialType.SBOM_CYCLONEDX_JSON;
    case 5:
    case "SBOM_SPDX_JSON":
      return CraftingSchema_Material_MaterialType.SBOM_SPDX_JSON;
    case 6:
    case "JUNIT_XML":
      return CraftingSchema_Material_MaterialType.JUNIT_XML;
    case 7:
    case "OPENVEX":
      return CraftingSchema_Material_MaterialType.OPENVEX;
    case 8:
    case "CSAF_VEX":
      return CraftingSchema_Material_MaterialType.CSAF_VEX;
    case 9:
    case "SARIF":
      return CraftingSchema_Material_MaterialType.SARIF;
    case 10:
    case "HELM_CHART":
      return CraftingSchema_Material_MaterialType.HELM_CHART;
    case 11:
    case "EVIDENCE":
      return CraftingSchema_Material_MaterialType.EVIDENCE;
    case -1:
    case "UNRECOGNIZED":
    default:
      return CraftingSchema_Material_MaterialType.UNRECOGNIZED;
  }
}

export function craftingSchema_Material_MaterialTypeToJSON(object: CraftingSchema_Material_MaterialType): string {
  switch (object) {
    case CraftingSchema_Material_MaterialType.MATERIAL_TYPE_UNSPECIFIED:
      return "MATERIAL_TYPE_UNSPECIFIED";
    case CraftingSchema_Material_MaterialType.STRING:
      return "STRING";
    case CraftingSchema_Material_MaterialType.CONTAINER_IMAGE:
      return "CONTAINER_IMAGE";
    case CraftingSchema_Material_MaterialType.ARTIFACT:
      return "ARTIFACT";
    case CraftingSchema_Material_MaterialType.SBOM_CYCLONEDX_JSON:
      return "SBOM_CYCLONEDX_JSON";
    case CraftingSchema_Material_MaterialType.SBOM_SPDX_JSON:
      return "SBOM_SPDX_JSON";
    case CraftingSchema_Material_MaterialType.JUNIT_XML:
      return "JUNIT_XML";
    case CraftingSchema_Material_MaterialType.OPENVEX:
      return "OPENVEX";
    case CraftingSchema_Material_MaterialType.CSAF_VEX:
      return "CSAF_VEX";
    case CraftingSchema_Material_MaterialType.SARIF:
      return "SARIF";
    case CraftingSchema_Material_MaterialType.HELM_CHART:
      return "HELM_CHART";
    case CraftingSchema_Material_MaterialType.EVIDENCE:
      return "EVIDENCE";
    case CraftingSchema_Material_MaterialType.UNRECOGNIZED:
    default:
      return "UNRECOGNIZED";
  }
}

export interface Annotation {
  /** Single word optionally separated with _ */
  name: string;
  /** This value can be set in the contract or provided during the attestation */
  value: string;
}

function createBaseCraftingSchema(): CraftingSchema {
  return { schemaVersion: "", materials: [], envAllowList: [], runner: undefined, annotations: [] };
}

export const CraftingSchema = {
  encode(message: CraftingSchema, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.schemaVersion !== "") {
      writer.uint32(10).string(message.schemaVersion);
    }
    for (const v of message.materials) {
      CraftingSchema_Material.encode(v!, writer.uint32(18).fork()).ldelim();
    }
    for (const v of message.envAllowList) {
      writer.uint32(26).string(v!);
    }
    if (message.runner !== undefined) {
      CraftingSchema_Runner.encode(message.runner, writer.uint32(34).fork()).ldelim();
    }
    for (const v of message.annotations) {
      Annotation.encode(v!, writer.uint32(42).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CraftingSchema {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCraftingSchema();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.schemaVersion = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.materials.push(CraftingSchema_Material.decode(reader, reader.uint32()));
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.envAllowList.push(reader.string());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.runner = CraftingSchema_Runner.decode(reader, reader.uint32());
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.annotations.push(Annotation.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CraftingSchema {
    return {
      schemaVersion: isSet(object.schemaVersion) ? String(object.schemaVersion) : "",
      materials: Array.isArray(object?.materials)
        ? object.materials.map((e: any) => CraftingSchema_Material.fromJSON(e))
        : [],
      envAllowList: Array.isArray(object?.envAllowList) ? object.envAllowList.map((e: any) => String(e)) : [],
      runner: isSet(object.runner) ? CraftingSchema_Runner.fromJSON(object.runner) : undefined,
      annotations: Array.isArray(object?.annotations) ? object.annotations.map((e: any) => Annotation.fromJSON(e)) : [],
    };
  },

  toJSON(message: CraftingSchema): unknown {
    const obj: any = {};
    message.schemaVersion !== undefined && (obj.schemaVersion = message.schemaVersion);
    if (message.materials) {
      obj.materials = message.materials.map((e) => e ? CraftingSchema_Material.toJSON(e) : undefined);
    } else {
      obj.materials = [];
    }
    if (message.envAllowList) {
      obj.envAllowList = message.envAllowList.map((e) => e);
    } else {
      obj.envAllowList = [];
    }
    message.runner !== undefined &&
      (obj.runner = message.runner ? CraftingSchema_Runner.toJSON(message.runner) : undefined);
    if (message.annotations) {
      obj.annotations = message.annotations.map((e) => e ? Annotation.toJSON(e) : undefined);
    } else {
      obj.annotations = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<CraftingSchema>, I>>(base?: I): CraftingSchema {
    return CraftingSchema.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CraftingSchema>, I>>(object: I): CraftingSchema {
    const message = createBaseCraftingSchema();
    message.schemaVersion = object.schemaVersion ?? "";
    message.materials = object.materials?.map((e) => CraftingSchema_Material.fromPartial(e)) || [];
    message.envAllowList = object.envAllowList?.map((e) => e) || [];
    message.runner = (object.runner !== undefined && object.runner !== null)
      ? CraftingSchema_Runner.fromPartial(object.runner)
      : undefined;
    message.annotations = object.annotations?.map((e) => Annotation.fromPartial(e)) || [];
    return message;
  },
};

function createBaseCraftingSchema_Runner(): CraftingSchema_Runner {
  return { type: 0 };
}

export const CraftingSchema_Runner = {
  encode(message: CraftingSchema_Runner, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.type !== 0) {
      writer.uint32(8).int32(message.type);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CraftingSchema_Runner {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCraftingSchema_Runner();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.type = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CraftingSchema_Runner {
    return { type: isSet(object.type) ? craftingSchema_Runner_RunnerTypeFromJSON(object.type) : 0 };
  },

  toJSON(message: CraftingSchema_Runner): unknown {
    const obj: any = {};
    message.type !== undefined && (obj.type = craftingSchema_Runner_RunnerTypeToJSON(message.type));
    return obj;
  },

  create<I extends Exact<DeepPartial<CraftingSchema_Runner>, I>>(base?: I): CraftingSchema_Runner {
    return CraftingSchema_Runner.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CraftingSchema_Runner>, I>>(object: I): CraftingSchema_Runner {
    const message = createBaseCraftingSchema_Runner();
    message.type = object.type ?? 0;
    return message;
  },
};

function createBaseCraftingSchema_Material(): CraftingSchema_Material {
  return { type: 0, name: "", optional: false, output: false, annotations: [] };
}

export const CraftingSchema_Material = {
  encode(message: CraftingSchema_Material, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.type !== 0) {
      writer.uint32(8).int32(message.type);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.optional === true) {
      writer.uint32(24).bool(message.optional);
    }
    if (message.output === true) {
      writer.uint32(32).bool(message.output);
    }
    for (const v of message.annotations) {
      Annotation.encode(v!, writer.uint32(42).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): CraftingSchema_Material {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCraftingSchema_Material();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 8) {
            break;
          }

          message.type = reader.int32() as any;
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.name = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.optional = reader.bool();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.output = reader.bool();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.annotations.push(Annotation.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): CraftingSchema_Material {
    return {
      type: isSet(object.type) ? craftingSchema_Material_MaterialTypeFromJSON(object.type) : 0,
      name: isSet(object.name) ? String(object.name) : "",
      optional: isSet(object.optional) ? Boolean(object.optional) : false,
      output: isSet(object.output) ? Boolean(object.output) : false,
      annotations: Array.isArray(object?.annotations) ? object.annotations.map((e: any) => Annotation.fromJSON(e)) : [],
    };
  },

  toJSON(message: CraftingSchema_Material): unknown {
    const obj: any = {};
    message.type !== undefined && (obj.type = craftingSchema_Material_MaterialTypeToJSON(message.type));
    message.name !== undefined && (obj.name = message.name);
    message.optional !== undefined && (obj.optional = message.optional);
    message.output !== undefined && (obj.output = message.output);
    if (message.annotations) {
      obj.annotations = message.annotations.map((e) => e ? Annotation.toJSON(e) : undefined);
    } else {
      obj.annotations = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<CraftingSchema_Material>, I>>(base?: I): CraftingSchema_Material {
    return CraftingSchema_Material.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<CraftingSchema_Material>, I>>(object: I): CraftingSchema_Material {
    const message = createBaseCraftingSchema_Material();
    message.type = object.type ?? 0;
    message.name = object.name ?? "";
    message.optional = object.optional ?? false;
    message.output = object.output ?? false;
    message.annotations = object.annotations?.map((e) => Annotation.fromPartial(e)) || [];
    return message;
  },
};

function createBaseAnnotation(): Annotation {
  return { name: "", value: "" };
}

export const Annotation = {
  encode(message: Annotation, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.value !== "") {
      writer.uint32(18).string(message.value);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Annotation {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAnnotation();
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

  fromJSON(object: any): Annotation {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      value: isSet(object.value) ? String(object.value) : "",
    };
  },

  toJSON(message: Annotation): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.value !== undefined && (obj.value = message.value);
    return obj;
  },

  create<I extends Exact<DeepPartial<Annotation>, I>>(base?: I): Annotation {
    return Annotation.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Annotation>, I>>(object: I): Annotation {
    const message = createBaseAnnotation();
    message.name = object.name ?? "";
    message.value = object.value ?? "";
    return message;
  },
};

type Builtin = Date | Function | Uint8Array | string | number | boolean | undefined;

export type DeepPartial<T> = T extends Builtin ? T
  : T extends Array<infer U> ? Array<DeepPartial<U>> : T extends ReadonlyArray<infer U> ? ReadonlyArray<DeepPartial<U>>
  : T extends {} ? { [K in keyof T]?: DeepPartial<T[K]> }
  : Partial<T>;

type KeysOfUnion<T> = T extends T ? keyof T : never;
export type Exact<P, I extends P> = P extends Builtin ? P
  : P & { [K in keyof P]: Exact<P[K], I[K]> } & { [K in Exclude<keyof I, KeysOfUnion<P>>]: never };

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}
