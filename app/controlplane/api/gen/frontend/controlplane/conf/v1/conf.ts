/* eslint-disable */
import _m0 from "protobufjs/minimal";
import { Credentials } from "../../../credentials/v1/config";
import { Duration } from "../../../google/protobuf/duration";

export const protobufPackage = "controlplane.conf.v1";

export interface Bootstrap {
  server?: Server;
  data?: Data;
  auth?: Auth;
  observability?: Bootstrap_Observability;
  credentialsService?: Credentials;
  /** CAS Server endpoint */
  casServer?: Bootstrap_CASServer;
  /**
   * Plugins directory
   * NOTE: plugins have the form of chainloop-plugin-<name>
   */
  pluginsDir: string;
}

export interface Bootstrap_Observability {
  sentry?: Bootstrap_Observability_Sentry;
}

export interface Bootstrap_Observability_Sentry {
  dsn: string;
  environment: string;
}

export interface Bootstrap_CASServer {
  grpc?: Server_GRPC;
  /**
   * insecure is used to disable TLS handshake
   * Only use for development purposes!
   */
  insecure: boolean;
}

export interface Server {
  http?: Server_HTTP;
  grpc?: Server_GRPC;
  /** HTTPMetrics defines the HTTP server that exposes prometheus metrics */
  httpMetrics?: Server_HTTP;
}

export interface Server_HTTP {
  network: string;
  addr: string;
  /**
   * In the form of [scheme]://[host] i.e https://instance.chainloop.dev
   * Optional
   */
  externalUrl: string;
  timeout?: Duration;
}

export interface Server_GRPC {
  network: string;
  addr: string;
  timeout?: Duration;
}

export interface Data {
  database?: Data_Database;
}

export interface Data_Database {
  driver: string;
  source: string;
}

export interface Auth {
  /** Authentication creates a JWT that uses this secret for signing */
  generatedJwsHmacSecret: string;
  allowList: string[];
  casRobotAccountPrivateKeyPath: string;
  oidc?: Auth_OIDC;
}

export interface Auth_OIDC {
  domain: string;
  clientId: string;
  clientSecret: string;
  redirectUrlScheme: string;
}

function createBaseBootstrap(): Bootstrap {
  return {
    server: undefined,
    data: undefined,
    auth: undefined,
    observability: undefined,
    credentialsService: undefined,
    casServer: undefined,
    pluginsDir: "",
  };
}

export const Bootstrap = {
  encode(message: Bootstrap, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.server !== undefined) {
      Server.encode(message.server, writer.uint32(10).fork()).ldelim();
    }
    if (message.data !== undefined) {
      Data.encode(message.data, writer.uint32(18).fork()).ldelim();
    }
    if (message.auth !== undefined) {
      Auth.encode(message.auth, writer.uint32(26).fork()).ldelim();
    }
    if (message.observability !== undefined) {
      Bootstrap_Observability.encode(message.observability, writer.uint32(34).fork()).ldelim();
    }
    if (message.credentialsService !== undefined) {
      Credentials.encode(message.credentialsService, writer.uint32(42).fork()).ldelim();
    }
    if (message.casServer !== undefined) {
      Bootstrap_CASServer.encode(message.casServer, writer.uint32(50).fork()).ldelim();
    }
    if (message.pluginsDir !== "") {
      writer.uint32(58).string(message.pluginsDir);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Bootstrap {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseBootstrap();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.server = Server.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.data = Data.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.auth = Auth.decode(reader, reader.uint32());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.observability = Bootstrap_Observability.decode(reader, reader.uint32());
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.credentialsService = Credentials.decode(reader, reader.uint32());
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.casServer = Bootstrap_CASServer.decode(reader, reader.uint32());
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.pluginsDir = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Bootstrap {
    return {
      server: isSet(object.server) ? Server.fromJSON(object.server) : undefined,
      data: isSet(object.data) ? Data.fromJSON(object.data) : undefined,
      auth: isSet(object.auth) ? Auth.fromJSON(object.auth) : undefined,
      observability: isSet(object.observability) ? Bootstrap_Observability.fromJSON(object.observability) : undefined,
      credentialsService: isSet(object.credentialsService)
        ? Credentials.fromJSON(object.credentialsService)
        : undefined,
      casServer: isSet(object.casServer) ? Bootstrap_CASServer.fromJSON(object.casServer) : undefined,
      pluginsDir: isSet(object.pluginsDir) ? String(object.pluginsDir) : "",
    };
  },

  toJSON(message: Bootstrap): unknown {
    const obj: any = {};
    message.server !== undefined && (obj.server = message.server ? Server.toJSON(message.server) : undefined);
    message.data !== undefined && (obj.data = message.data ? Data.toJSON(message.data) : undefined);
    message.auth !== undefined && (obj.auth = message.auth ? Auth.toJSON(message.auth) : undefined);
    message.observability !== undefined &&
      (obj.observability = message.observability ? Bootstrap_Observability.toJSON(message.observability) : undefined);
    message.credentialsService !== undefined &&
      (obj.credentialsService = message.credentialsService
        ? Credentials.toJSON(message.credentialsService)
        : undefined);
    message.casServer !== undefined &&
      (obj.casServer = message.casServer ? Bootstrap_CASServer.toJSON(message.casServer) : undefined);
    message.pluginsDir !== undefined && (obj.pluginsDir = message.pluginsDir);
    return obj;
  },

  create<I extends Exact<DeepPartial<Bootstrap>, I>>(base?: I): Bootstrap {
    return Bootstrap.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Bootstrap>, I>>(object: I): Bootstrap {
    const message = createBaseBootstrap();
    message.server = (object.server !== undefined && object.server !== null)
      ? Server.fromPartial(object.server)
      : undefined;
    message.data = (object.data !== undefined && object.data !== null) ? Data.fromPartial(object.data) : undefined;
    message.auth = (object.auth !== undefined && object.auth !== null) ? Auth.fromPartial(object.auth) : undefined;
    message.observability = (object.observability !== undefined && object.observability !== null)
      ? Bootstrap_Observability.fromPartial(object.observability)
      : undefined;
    message.credentialsService = (object.credentialsService !== undefined && object.credentialsService !== null)
      ? Credentials.fromPartial(object.credentialsService)
      : undefined;
    message.casServer = (object.casServer !== undefined && object.casServer !== null)
      ? Bootstrap_CASServer.fromPartial(object.casServer)
      : undefined;
    message.pluginsDir = object.pluginsDir ?? "";
    return message;
  },
};

function createBaseBootstrap_Observability(): Bootstrap_Observability {
  return { sentry: undefined };
}

export const Bootstrap_Observability = {
  encode(message: Bootstrap_Observability, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.sentry !== undefined) {
      Bootstrap_Observability_Sentry.encode(message.sentry, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Bootstrap_Observability {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseBootstrap_Observability();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.sentry = Bootstrap_Observability_Sentry.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Bootstrap_Observability {
    return { sentry: isSet(object.sentry) ? Bootstrap_Observability_Sentry.fromJSON(object.sentry) : undefined };
  },

  toJSON(message: Bootstrap_Observability): unknown {
    const obj: any = {};
    message.sentry !== undefined &&
      (obj.sentry = message.sentry ? Bootstrap_Observability_Sentry.toJSON(message.sentry) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Bootstrap_Observability>, I>>(base?: I): Bootstrap_Observability {
    return Bootstrap_Observability.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Bootstrap_Observability>, I>>(object: I): Bootstrap_Observability {
    const message = createBaseBootstrap_Observability();
    message.sentry = (object.sentry !== undefined && object.sentry !== null)
      ? Bootstrap_Observability_Sentry.fromPartial(object.sentry)
      : undefined;
    return message;
  },
};

function createBaseBootstrap_Observability_Sentry(): Bootstrap_Observability_Sentry {
  return { dsn: "", environment: "" };
}

export const Bootstrap_Observability_Sentry = {
  encode(message: Bootstrap_Observability_Sentry, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.dsn !== "") {
      writer.uint32(10).string(message.dsn);
    }
    if (message.environment !== "") {
      writer.uint32(18).string(message.environment);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Bootstrap_Observability_Sentry {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseBootstrap_Observability_Sentry();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.dsn = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.environment = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Bootstrap_Observability_Sentry {
    return {
      dsn: isSet(object.dsn) ? String(object.dsn) : "",
      environment: isSet(object.environment) ? String(object.environment) : "",
    };
  },

  toJSON(message: Bootstrap_Observability_Sentry): unknown {
    const obj: any = {};
    message.dsn !== undefined && (obj.dsn = message.dsn);
    message.environment !== undefined && (obj.environment = message.environment);
    return obj;
  },

  create<I extends Exact<DeepPartial<Bootstrap_Observability_Sentry>, I>>(base?: I): Bootstrap_Observability_Sentry {
    return Bootstrap_Observability_Sentry.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Bootstrap_Observability_Sentry>, I>>(
    object: I,
  ): Bootstrap_Observability_Sentry {
    const message = createBaseBootstrap_Observability_Sentry();
    message.dsn = object.dsn ?? "";
    message.environment = object.environment ?? "";
    return message;
  },
};

function createBaseBootstrap_CASServer(): Bootstrap_CASServer {
  return { grpc: undefined, insecure: false };
}

export const Bootstrap_CASServer = {
  encode(message: Bootstrap_CASServer, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.grpc !== undefined) {
      Server_GRPC.encode(message.grpc, writer.uint32(10).fork()).ldelim();
    }
    if (message.insecure === true) {
      writer.uint32(16).bool(message.insecure);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Bootstrap_CASServer {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseBootstrap_CASServer();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.grpc = Server_GRPC.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.insecure = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Bootstrap_CASServer {
    return {
      grpc: isSet(object.grpc) ? Server_GRPC.fromJSON(object.grpc) : undefined,
      insecure: isSet(object.insecure) ? Boolean(object.insecure) : false,
    };
  },

  toJSON(message: Bootstrap_CASServer): unknown {
    const obj: any = {};
    message.grpc !== undefined && (obj.grpc = message.grpc ? Server_GRPC.toJSON(message.grpc) : undefined);
    message.insecure !== undefined && (obj.insecure = message.insecure);
    return obj;
  },

  create<I extends Exact<DeepPartial<Bootstrap_CASServer>, I>>(base?: I): Bootstrap_CASServer {
    return Bootstrap_CASServer.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Bootstrap_CASServer>, I>>(object: I): Bootstrap_CASServer {
    const message = createBaseBootstrap_CASServer();
    message.grpc = (object.grpc !== undefined && object.grpc !== null)
      ? Server_GRPC.fromPartial(object.grpc)
      : undefined;
    message.insecure = object.insecure ?? false;
    return message;
  },
};

function createBaseServer(): Server {
  return { http: undefined, grpc: undefined, httpMetrics: undefined };
}

export const Server = {
  encode(message: Server, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.http !== undefined) {
      Server_HTTP.encode(message.http, writer.uint32(10).fork()).ldelim();
    }
    if (message.grpc !== undefined) {
      Server_GRPC.encode(message.grpc, writer.uint32(18).fork()).ldelim();
    }
    if (message.httpMetrics !== undefined) {
      Server_HTTP.encode(message.httpMetrics, writer.uint32(26).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Server {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseServer();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.http = Server_HTTP.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.grpc = Server_GRPC.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.httpMetrics = Server_HTTP.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Server {
    return {
      http: isSet(object.http) ? Server_HTTP.fromJSON(object.http) : undefined,
      grpc: isSet(object.grpc) ? Server_GRPC.fromJSON(object.grpc) : undefined,
      httpMetrics: isSet(object.httpMetrics) ? Server_HTTP.fromJSON(object.httpMetrics) : undefined,
    };
  },

  toJSON(message: Server): unknown {
    const obj: any = {};
    message.http !== undefined && (obj.http = message.http ? Server_HTTP.toJSON(message.http) : undefined);
    message.grpc !== undefined && (obj.grpc = message.grpc ? Server_GRPC.toJSON(message.grpc) : undefined);
    message.httpMetrics !== undefined &&
      (obj.httpMetrics = message.httpMetrics ? Server_HTTP.toJSON(message.httpMetrics) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Server>, I>>(base?: I): Server {
    return Server.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Server>, I>>(object: I): Server {
    const message = createBaseServer();
    message.http = (object.http !== undefined && object.http !== null)
      ? Server_HTTP.fromPartial(object.http)
      : undefined;
    message.grpc = (object.grpc !== undefined && object.grpc !== null)
      ? Server_GRPC.fromPartial(object.grpc)
      : undefined;
    message.httpMetrics = (object.httpMetrics !== undefined && object.httpMetrics !== null)
      ? Server_HTTP.fromPartial(object.httpMetrics)
      : undefined;
    return message;
  },
};

function createBaseServer_HTTP(): Server_HTTP {
  return { network: "", addr: "", externalUrl: "", timeout: undefined };
}

export const Server_HTTP = {
  encode(message: Server_HTTP, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.network !== "") {
      writer.uint32(10).string(message.network);
    }
    if (message.addr !== "") {
      writer.uint32(18).string(message.addr);
    }
    if (message.externalUrl !== "") {
      writer.uint32(34).string(message.externalUrl);
    }
    if (message.timeout !== undefined) {
      Duration.encode(message.timeout, writer.uint32(42).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Server_HTTP {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseServer_HTTP();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.network = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.addr = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.externalUrl = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.timeout = Duration.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Server_HTTP {
    return {
      network: isSet(object.network) ? String(object.network) : "",
      addr: isSet(object.addr) ? String(object.addr) : "",
      externalUrl: isSet(object.externalUrl) ? String(object.externalUrl) : "",
      timeout: isSet(object.timeout) ? Duration.fromJSON(object.timeout) : undefined,
    };
  },

  toJSON(message: Server_HTTP): unknown {
    const obj: any = {};
    message.network !== undefined && (obj.network = message.network);
    message.addr !== undefined && (obj.addr = message.addr);
    message.externalUrl !== undefined && (obj.externalUrl = message.externalUrl);
    message.timeout !== undefined && (obj.timeout = message.timeout ? Duration.toJSON(message.timeout) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Server_HTTP>, I>>(base?: I): Server_HTTP {
    return Server_HTTP.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Server_HTTP>, I>>(object: I): Server_HTTP {
    const message = createBaseServer_HTTP();
    message.network = object.network ?? "";
    message.addr = object.addr ?? "";
    message.externalUrl = object.externalUrl ?? "";
    message.timeout = (object.timeout !== undefined && object.timeout !== null)
      ? Duration.fromPartial(object.timeout)
      : undefined;
    return message;
  },
};

function createBaseServer_GRPC(): Server_GRPC {
  return { network: "", addr: "", timeout: undefined };
}

export const Server_GRPC = {
  encode(message: Server_GRPC, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.network !== "") {
      writer.uint32(10).string(message.network);
    }
    if (message.addr !== "") {
      writer.uint32(18).string(message.addr);
    }
    if (message.timeout !== undefined) {
      Duration.encode(message.timeout, writer.uint32(26).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Server_GRPC {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseServer_GRPC();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.network = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.addr = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.timeout = Duration.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Server_GRPC {
    return {
      network: isSet(object.network) ? String(object.network) : "",
      addr: isSet(object.addr) ? String(object.addr) : "",
      timeout: isSet(object.timeout) ? Duration.fromJSON(object.timeout) : undefined,
    };
  },

  toJSON(message: Server_GRPC): unknown {
    const obj: any = {};
    message.network !== undefined && (obj.network = message.network);
    message.addr !== undefined && (obj.addr = message.addr);
    message.timeout !== undefined && (obj.timeout = message.timeout ? Duration.toJSON(message.timeout) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Server_GRPC>, I>>(base?: I): Server_GRPC {
    return Server_GRPC.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Server_GRPC>, I>>(object: I): Server_GRPC {
    const message = createBaseServer_GRPC();
    message.network = object.network ?? "";
    message.addr = object.addr ?? "";
    message.timeout = (object.timeout !== undefined && object.timeout !== null)
      ? Duration.fromPartial(object.timeout)
      : undefined;
    return message;
  },
};

function createBaseData(): Data {
  return { database: undefined };
}

export const Data = {
  encode(message: Data, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.database !== undefined) {
      Data_Database.encode(message.database, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Data {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseData();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.database = Data_Database.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Data {
    return { database: isSet(object.database) ? Data_Database.fromJSON(object.database) : undefined };
  },

  toJSON(message: Data): unknown {
    const obj: any = {};
    message.database !== undefined &&
      (obj.database = message.database ? Data_Database.toJSON(message.database) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Data>, I>>(base?: I): Data {
    return Data.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Data>, I>>(object: I): Data {
    const message = createBaseData();
    message.database = (object.database !== undefined && object.database !== null)
      ? Data_Database.fromPartial(object.database)
      : undefined;
    return message;
  },
};

function createBaseData_Database(): Data_Database {
  return { driver: "", source: "" };
}

export const Data_Database = {
  encode(message: Data_Database, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.driver !== "") {
      writer.uint32(10).string(message.driver);
    }
    if (message.source !== "") {
      writer.uint32(18).string(message.source);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Data_Database {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseData_Database();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.driver = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.source = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Data_Database {
    return {
      driver: isSet(object.driver) ? String(object.driver) : "",
      source: isSet(object.source) ? String(object.source) : "",
    };
  },

  toJSON(message: Data_Database): unknown {
    const obj: any = {};
    message.driver !== undefined && (obj.driver = message.driver);
    message.source !== undefined && (obj.source = message.source);
    return obj;
  },

  create<I extends Exact<DeepPartial<Data_Database>, I>>(base?: I): Data_Database {
    return Data_Database.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Data_Database>, I>>(object: I): Data_Database {
    const message = createBaseData_Database();
    message.driver = object.driver ?? "";
    message.source = object.source ?? "";
    return message;
  },
};

function createBaseAuth(): Auth {
  return { generatedJwsHmacSecret: "", allowList: [], casRobotAccountPrivateKeyPath: "", oidc: undefined };
}

export const Auth = {
  encode(message: Auth, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.generatedJwsHmacSecret !== "") {
      writer.uint32(18).string(message.generatedJwsHmacSecret);
    }
    for (const v of message.allowList) {
      writer.uint32(26).string(v!);
    }
    if (message.casRobotAccountPrivateKeyPath !== "") {
      writer.uint32(34).string(message.casRobotAccountPrivateKeyPath);
    }
    if (message.oidc !== undefined) {
      Auth_OIDC.encode(message.oidc, writer.uint32(50).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Auth {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAuth();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 2:
          if (tag !== 18) {
            break;
          }

          message.generatedJwsHmacSecret = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.allowList.push(reader.string());
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.casRobotAccountPrivateKeyPath = reader.string();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.oidc = Auth_OIDC.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Auth {
    return {
      generatedJwsHmacSecret: isSet(object.generatedJwsHmacSecret) ? String(object.generatedJwsHmacSecret) : "",
      allowList: Array.isArray(object?.allowList) ? object.allowList.map((e: any) => String(e)) : [],
      casRobotAccountPrivateKeyPath: isSet(object.casRobotAccountPrivateKeyPath)
        ? String(object.casRobotAccountPrivateKeyPath)
        : "",
      oidc: isSet(object.oidc) ? Auth_OIDC.fromJSON(object.oidc) : undefined,
    };
  },

  toJSON(message: Auth): unknown {
    const obj: any = {};
    message.generatedJwsHmacSecret !== undefined && (obj.generatedJwsHmacSecret = message.generatedJwsHmacSecret);
    if (message.allowList) {
      obj.allowList = message.allowList.map((e) => e);
    } else {
      obj.allowList = [];
    }
    message.casRobotAccountPrivateKeyPath !== undefined &&
      (obj.casRobotAccountPrivateKeyPath = message.casRobotAccountPrivateKeyPath);
    message.oidc !== undefined && (obj.oidc = message.oidc ? Auth_OIDC.toJSON(message.oidc) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<Auth>, I>>(base?: I): Auth {
    return Auth.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Auth>, I>>(object: I): Auth {
    const message = createBaseAuth();
    message.generatedJwsHmacSecret = object.generatedJwsHmacSecret ?? "";
    message.allowList = object.allowList?.map((e) => e) || [];
    message.casRobotAccountPrivateKeyPath = object.casRobotAccountPrivateKeyPath ?? "";
    message.oidc = (object.oidc !== undefined && object.oidc !== null) ? Auth_OIDC.fromPartial(object.oidc) : undefined;
    return message;
  },
};

function createBaseAuth_OIDC(): Auth_OIDC {
  return { domain: "", clientId: "", clientSecret: "", redirectUrlScheme: "" };
}

export const Auth_OIDC = {
  encode(message: Auth_OIDC, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.domain !== "") {
      writer.uint32(10).string(message.domain);
    }
    if (message.clientId !== "") {
      writer.uint32(18).string(message.clientId);
    }
    if (message.clientSecret !== "") {
      writer.uint32(26).string(message.clientSecret);
    }
    if (message.redirectUrlScheme !== "") {
      writer.uint32(34).string(message.redirectUrlScheme);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): Auth_OIDC {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAuth_OIDC();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.domain = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.clientId = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.clientSecret = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.redirectUrlScheme = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): Auth_OIDC {
    return {
      domain: isSet(object.domain) ? String(object.domain) : "",
      clientId: isSet(object.clientId) ? String(object.clientId) : "",
      clientSecret: isSet(object.clientSecret) ? String(object.clientSecret) : "",
      redirectUrlScheme: isSet(object.redirectUrlScheme) ? String(object.redirectUrlScheme) : "",
    };
  },

  toJSON(message: Auth_OIDC): unknown {
    const obj: any = {};
    message.domain !== undefined && (obj.domain = message.domain);
    message.clientId !== undefined && (obj.clientId = message.clientId);
    message.clientSecret !== undefined && (obj.clientSecret = message.clientSecret);
    message.redirectUrlScheme !== undefined && (obj.redirectUrlScheme = message.redirectUrlScheme);
    return obj;
  },

  create<I extends Exact<DeepPartial<Auth_OIDC>, I>>(base?: I): Auth_OIDC {
    return Auth_OIDC.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<Auth_OIDC>, I>>(object: I): Auth_OIDC {
    const message = createBaseAuth_OIDC();
    message.domain = object.domain ?? "";
    message.clientId = object.clientId ?? "";
    message.clientSecret = object.clientSecret ?? "";
    message.redirectUrlScheme = object.redirectUrlScheme ?? "";
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
