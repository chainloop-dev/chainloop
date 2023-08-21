/* eslint-disable */
import _m0 from "protobufjs/minimal";

export const protobufPackage = "credentials.v1";

/** Where the credentials to access the backends are stored */
export interface Credentials {
  awsSecretManager?: Credentials_AWSSecretManager | undefined;
  vault?: Credentials_Vault | undefined;
  gcpSecretManager?: Credentials_GCPSecretManager | undefined;
}

/** Top level is deprecated now */
export interface Credentials_AWSSecretManager {
  creds?: Credentials_AWSSecretManager_Creds;
  region: string;
}

export interface Credentials_AWSSecretManager_Creds {
  accessKey: string;
  secretKey: string;
}

export interface Credentials_Vault {
  /** TODO: Use application role auth instead */
  token: string;
  /**
   * Instance address, including port
   * i.e "http://127.0.0.1:8200"
   */
  address: string;
  /** mount path of the kv engine, default /secret */
  mountPath: string;
}

export interface Credentials_GCPSecretManager {
  /** project number */
  projectId: string;
  /** path to service account key in json format */
  serviceAccountKey: string;
}

function createBaseCredentials(): Credentials {
  return { awsSecretManager: undefined, vault: undefined, gcpSecretManager: undefined };
}

export const Credentials = {
  encode(message: Credentials, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.awsSecretManager !== undefined) {
      Credentials_AWSSecretManager.encode(message.awsSecretManager, writer.uint32(10).fork()).ldelim();
    }
    if (message.vault !== undefined) {
      Credentials_Vault.encode(message.vault, writer.uint32(18).fork()).ldelim();
    }
    if (message.gcpSecretManager !== undefined) {
      Credentials_GCPSecretManager.encode(message.gcpSecretManager, writer.uint32(26).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Credentials {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCredentials();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.awsSecretManager = Credentials_AWSSecretManager.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.vault = Credentials_Vault.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.gcpSecretManager = Credentials_GCPSecretManager.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Credentials {
    return {
      awsSecretManager: isSet(object.awsSecretManager)
        ? Credentials_AWSSecretManager.fromJSON(object.awsSecretManager)
        : undefined,
      vault: isSet(object.vault) ? Credentials_Vault.fromJSON(object.vault) : undefined,
      gcpSecretManager: isSet(object.gcpSecretManager)
        ? Credentials_GCPSecretManager.fromJSON(object.gcpSecretManager)
        : undefined,
    };
  },

  toJSON(message: Credentials): unknown {
    const obj: any = {};
    message.awsSecretManager !== undefined && (obj.awsSecretManager = message.awsSecretManager
      ? Credentials_AWSSecretManager.toJSON(message.awsSecretManager)
      : undefined);
    message.vault !== undefined && (obj.vault = message.vault ? Credentials_Vault.toJSON(message.vault) : undefined);
    message.gcpSecretManager !== undefined && (obj.gcpSecretManager = message.gcpSecretManager
      ? Credentials_GCPSecretManager.toJSON(message.gcpSecretManager)
      : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Credentials>, I>>(base?: I): Credentials {
    return Credentials.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Credentials>, I>>(object: I): Credentials {
    const message = createBaseCredentials();
    message.awsSecretManager = (object.awsSecretManager !== undefined && object.awsSecretManager !== null)
      ? Credentials_AWSSecretManager.fromPartial(object.awsSecretManager)
      : undefined;
    message.vault = (object.vault !== undefined && object.vault !== null)
      ? Credentials_Vault.fromPartial(object.vault)
      : undefined;
    message.gcpSecretManager = (object.gcpSecretManager !== undefined && object.gcpSecretManager !== null)
      ? Credentials_GCPSecretManager.fromPartial(object.gcpSecretManager)
      : undefined;
    return message;
  },
};

function createBaseCredentials_AWSSecretManager(): Credentials_AWSSecretManager {
  return { creds: undefined, region: "" };
}

export const Credentials_AWSSecretManager = {
  encode(message: Credentials_AWSSecretManager, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.creds !== undefined) {
      Credentials_AWSSecretManager_Creds.encode(message.creds, writer.uint32(10).fork()).ldelim();
    }
    if (message.region !== "") {
      writer.uint32(18).string(message.region);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Credentials_AWSSecretManager {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCredentials_AWSSecretManager();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.creds = Credentials_AWSSecretManager_Creds.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.region = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Credentials_AWSSecretManager {
    return {
      creds: isSet(object.creds) ? Credentials_AWSSecretManager_Creds.fromJSON(object.creds) : undefined,
      region: isSet(object.region) ? String(object.region) : "",
    };
  },

  toJSON(message: Credentials_AWSSecretManager): unknown {
    const obj: any = {};
    message.creds !== undefined &&
      (obj.creds = message.creds ? Credentials_AWSSecretManager_Creds.toJSON(message.creds) : undefined);
    message.region !== undefined && (obj.region = message.region);
    return obj;
  },

  create<I extends Exact<DeepPartial<Credentials_AWSSecretManager>, I>>(base?: I): Credentials_AWSSecretManager {
    return Credentials_AWSSecretManager.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Credentials_AWSSecretManager>, I>>(object: I): Credentials_AWSSecretManager {
    const message = createBaseCredentials_AWSSecretManager();
    message.creds = (object.creds !== undefined && object.creds !== null)
      ? Credentials_AWSSecretManager_Creds.fromPartial(object.creds)
      : undefined;
    message.region = object.region ?? "";
    return message;
  },
};

function createBaseCredentials_AWSSecretManager_Creds(): Credentials_AWSSecretManager_Creds {
  return { accessKey: "", secretKey: "" };
}

export const Credentials_AWSSecretManager_Creds = {
  encode(message: Credentials_AWSSecretManager_Creds, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.accessKey !== "") {
      writer.uint32(10).string(message.accessKey);
    }
    if (message.secretKey !== "") {
      writer.uint32(18).string(message.secretKey);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Credentials_AWSSecretManager_Creds {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCredentials_AWSSecretManager_Creds();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.accessKey = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.secretKey = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Credentials_AWSSecretManager_Creds {
    return {
      accessKey: isSet(object.accessKey) ? String(object.accessKey) : "",
      secretKey: isSet(object.secretKey) ? String(object.secretKey) : "",
    };
  },

  toJSON(message: Credentials_AWSSecretManager_Creds): unknown {
    const obj: any = {};
    message.accessKey !== undefined && (obj.accessKey = message.accessKey);
    message.secretKey !== undefined && (obj.secretKey = message.secretKey);
    return obj;
  },

  create<I extends Exact<DeepPartial<Credentials_AWSSecretManager_Creds>, I>>(
    base?: I,
  ): Credentials_AWSSecretManager_Creds {
    return Credentials_AWSSecretManager_Creds.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Credentials_AWSSecretManager_Creds>, I>>(
    object: I,
  ): Credentials_AWSSecretManager_Creds {
    const message = createBaseCredentials_AWSSecretManager_Creds();
    message.accessKey = object.accessKey ?? "";
    message.secretKey = object.secretKey ?? "";
    return message;
  },
};

function createBaseCredentials_Vault(): Credentials_Vault {
  return { token: "", address: "", mountPath: "" };
}

export const Credentials_Vault = {
  encode(message: Credentials_Vault, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.token !== "") {
      writer.uint32(10).string(message.token);
    }
    if (message.address !== "") {
      writer.uint32(18).string(message.address);
    }
    if (message.mountPath !== "") {
      writer.uint32(26).string(message.mountPath);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Credentials_Vault {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCredentials_Vault();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.token = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.address = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.mountPath = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Credentials_Vault {
    return {
      token: isSet(object.token) ? String(object.token) : "",
      address: isSet(object.address) ? String(object.address) : "",
      mountPath: isSet(object.mountPath) ? String(object.mountPath) : "",
    };
  },

  toJSON(message: Credentials_Vault): unknown {
    const obj: any = {};
    message.token !== undefined && (obj.token = message.token);
    message.address !== undefined && (obj.address = message.address);
    message.mountPath !== undefined && (obj.mountPath = message.mountPath);
    return obj;
  },

  create<I extends Exact<DeepPartial<Credentials_Vault>, I>>(base?: I): Credentials_Vault {
    return Credentials_Vault.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Credentials_Vault>, I>>(object: I): Credentials_Vault {
    const message = createBaseCredentials_Vault();
    message.token = object.token ?? "";
    message.address = object.address ?? "";
    message.mountPath = object.mountPath ?? "";
    return message;
  },
};

function createBaseCredentials_GCPSecretManager(): Credentials_GCPSecretManager {
  return { projectId: "", serviceAccountKey: "" };
}

export const Credentials_GCPSecretManager = {
  encode(message: Credentials_GCPSecretManager, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.projectId !== "") {
      writer.uint32(10).string(message.projectId);
    }
    if (message.serviceAccountKey !== "") {
      writer.uint32(18).string(message.serviceAccountKey);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Credentials_GCPSecretManager {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseCredentials_GCPSecretManager();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.projectId = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.serviceAccountKey = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Credentials_GCPSecretManager {
    return {
      projectId: isSet(object.projectId) ? String(object.projectId) : "",
      serviceAccountKey: isSet(object.serviceAccountKey) ? String(object.serviceAccountKey) : "",
    };
  },

  toJSON(message: Credentials_GCPSecretManager): unknown {
    const obj: any = {};
    message.projectId !== undefined && (obj.projectId = message.projectId);
    message.serviceAccountKey !== undefined && (obj.serviceAccountKey = message.serviceAccountKey);
    return obj;
  },

  create<I extends Exact<DeepPartial<Credentials_GCPSecretManager>, I>>(base?: I): Credentials_GCPSecretManager {
    return Credentials_GCPSecretManager.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Credentials_GCPSecretManager>, I>>(object: I): Credentials_GCPSecretManager {
    const message = createBaseCredentials_GCPSecretManager();
    message.projectId = object.projectId ?? "";
    message.serviceAccountKey = object.serviceAccountKey ?? "";
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
