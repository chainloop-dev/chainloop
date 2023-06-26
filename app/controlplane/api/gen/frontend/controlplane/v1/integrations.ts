/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Struct } from "../../google/protobuf/struct";
import { Timestamp } from "../../google/protobuf/timestamp";
import { WorkflowItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface IntegrationsServiceRegisterRequest {
  /**
   * Kind of integration to register
   * This should match the ID of an existing plugin
   */
  pluginId: string;
  /** Arbitrary configuration for the integration */
  config?: { [key: string]: any };
  /** Description of the registration, used for display purposes */
  description: string;
}

export interface IntegrationsServiceRegisterResponse {
  result?: RegisteredIntegrationItem;
}

export interface IntegrationsServiceAttachRequest {
  /** UUID of the workflow to attach */
  workflowId: string;
  /** UUID of the integration registration to attach */
  integrationId: string;
  /** Arbitrary configuration for the integration */
  config?: { [key: string]: any };
}

export interface IntegrationsServiceAttachResponse {
  result?: IntegrationAttachmentItem;
}

export interface IntegrationsServiceListAvailableRequest {
}

export interface IntegrationsServiceListAvailableResponse {
  result: IntegrationAvailableItem[];
}

export interface IntegrationAvailableItem {
  /** Integration identifier */
  id: string;
  version: string;
  description: string;
  fanout?: PluginFanout | undefined;
}

/** PluginFanout describes a plugin that can be used to fanout attestation and materials to multiple integrations */
export interface PluginFanout {
  /** Registration JSON schema */
  registrationSchema: Uint8Array;
  /** Attachment JSON schema */
  attachmentSchema: Uint8Array;
  /** List of materials that the integration is subscribed to */
  subscribedMaterials: string[];
}

export interface IntegrationsServiceListRegistrationsRequest {
}

export interface IntegrationsServiceListRegistrationsResponse {
  result: RegisteredIntegrationItem[];
}

export interface IntegrationsServiceDescribeRegistrationRequest {
  id: string;
}

export interface IntegrationsServiceDescribeRegistrationResponse {
  result?: RegisteredIntegrationItem;
}

export interface IntegrationsServiceDetachRequest {
  id: string;
}

export interface IntegrationsServiceDetachResponse {
}

export interface ListAttachmentsRequest {
  /** Filter by workflow */
  workflowId: string;
}

export interface ListAttachmentsResponse {
  result: IntegrationAttachmentItem[];
}

export interface RegisteredIntegrationItem {
  id: string;
  kind: string;
  /** Description of the registration, used for display purposes */
  description: string;
  createdAt?: Date;
  /** Arbitrary configuration for the integration */
  config: Uint8Array;
}

export interface IntegrationAttachmentItem {
  id: string;
  createdAt?: Date;
  /** Arbitrary configuration for the attachment */
  config: Uint8Array;
  integration?: RegisteredIntegrationItem;
  workflow?: WorkflowItem;
}

export interface IntegrationsServiceDeregisterRequest {
  id: string;
}

export interface IntegrationsServiceDeregisterResponse {
}

function createBaseIntegrationsServiceRegisterRequest(): IntegrationsServiceRegisterRequest {
  return { pluginId: "", config: undefined, description: "" };
}

export const IntegrationsServiceRegisterRequest = {
  encode(message: IntegrationsServiceRegisterRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.pluginId !== "") {
      writer.uint32(10).string(message.pluginId);
    }
    if (message.config !== undefined) {
      Struct.encode(Struct.wrap(message.config), writer.uint32(26).fork()).ldelim();
    }
    if (message.description !== "") {
      writer.uint32(34).string(message.description);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceRegisterRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceRegisterRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.pluginId = reader.string();
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          message.config = Struct.unwrap(Struct.decode(reader, reader.uint32()));
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.description = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceRegisterRequest {
    return {
      pluginId: isSet(object.pluginId) ? String(object.pluginId) : "",
      config: isObject(object.config) ? object.config : undefined,
      description: isSet(object.description) ? String(object.description) : "",
    };
  },

  toJSON(message: IntegrationsServiceRegisterRequest): unknown {
    const obj: any = {};
    message.pluginId !== undefined && (obj.pluginId = message.pluginId);
    message.config !== undefined && (obj.config = message.config);
    message.description !== undefined && (obj.description = message.description);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceRegisterRequest>, I>>(
    base?: I,
  ): IntegrationsServiceRegisterRequest {
    return IntegrationsServiceRegisterRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceRegisterRequest>, I>>(
    object: I,
  ): IntegrationsServiceRegisterRequest {
    const message = createBaseIntegrationsServiceRegisterRequest();
    message.pluginId = object.pluginId ?? "";
    message.config = object.config ?? undefined;
    message.description = object.description ?? "";
    return message;
  },
};

function createBaseIntegrationsServiceRegisterResponse(): IntegrationsServiceRegisterResponse {
  return { result: undefined };
}

export const IntegrationsServiceRegisterResponse = {
  encode(message: IntegrationsServiceRegisterResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      RegisteredIntegrationItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceRegisterResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceRegisterResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = RegisteredIntegrationItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceRegisterResponse {
    return { result: isSet(object.result) ? RegisteredIntegrationItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: IntegrationsServiceRegisterResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? RegisteredIntegrationItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceRegisterResponse>, I>>(
    base?: I,
  ): IntegrationsServiceRegisterResponse {
    return IntegrationsServiceRegisterResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceRegisterResponse>, I>>(
    object: I,
  ): IntegrationsServiceRegisterResponse {
    const message = createBaseIntegrationsServiceRegisterResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? RegisteredIntegrationItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseIntegrationsServiceAttachRequest(): IntegrationsServiceAttachRequest {
  return { workflowId: "", integrationId: "", config: undefined };
}

export const IntegrationsServiceAttachRequest = {
  encode(message: IntegrationsServiceAttachRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowId !== "") {
      writer.uint32(10).string(message.workflowId);
    }
    if (message.integrationId !== "") {
      writer.uint32(18).string(message.integrationId);
    }
    if (message.config !== undefined) {
      Struct.encode(Struct.wrap(message.config), writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceAttachRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceAttachRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.workflowId = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.integrationId = reader.string();
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.config = Struct.unwrap(Struct.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceAttachRequest {
    return {
      workflowId: isSet(object.workflowId) ? String(object.workflowId) : "",
      integrationId: isSet(object.integrationId) ? String(object.integrationId) : "",
      config: isObject(object.config) ? object.config : undefined,
    };
  },

  toJSON(message: IntegrationsServiceAttachRequest): unknown {
    const obj: any = {};
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    message.integrationId !== undefined && (obj.integrationId = message.integrationId);
    message.config !== undefined && (obj.config = message.config);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceAttachRequest>, I>>(
    base?: I,
  ): IntegrationsServiceAttachRequest {
    return IntegrationsServiceAttachRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceAttachRequest>, I>>(
    object: I,
  ): IntegrationsServiceAttachRequest {
    const message = createBaseIntegrationsServiceAttachRequest();
    message.workflowId = object.workflowId ?? "";
    message.integrationId = object.integrationId ?? "";
    message.config = object.config ?? undefined;
    return message;
  },
};

function createBaseIntegrationsServiceAttachResponse(): IntegrationsServiceAttachResponse {
  return { result: undefined };
}

export const IntegrationsServiceAttachResponse = {
  encode(message: IntegrationsServiceAttachResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      IntegrationAttachmentItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceAttachResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceAttachResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = IntegrationAttachmentItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceAttachResponse {
    return { result: isSet(object.result) ? IntegrationAttachmentItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: IntegrationsServiceAttachResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? IntegrationAttachmentItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceAttachResponse>, I>>(
    base?: I,
  ): IntegrationsServiceAttachResponse {
    return IntegrationsServiceAttachResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceAttachResponse>, I>>(
    object: I,
  ): IntegrationsServiceAttachResponse {
    const message = createBaseIntegrationsServiceAttachResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? IntegrationAttachmentItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseIntegrationsServiceListAvailableRequest(): IntegrationsServiceListAvailableRequest {
  return {};
}

export const IntegrationsServiceListAvailableRequest = {
  encode(_: IntegrationsServiceListAvailableRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceListAvailableRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceListAvailableRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): IntegrationsServiceListAvailableRequest {
    return {};
  },

  toJSON(_: IntegrationsServiceListAvailableRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceListAvailableRequest>, I>>(
    base?: I,
  ): IntegrationsServiceListAvailableRequest {
    return IntegrationsServiceListAvailableRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceListAvailableRequest>, I>>(
    _: I,
  ): IntegrationsServiceListAvailableRequest {
    const message = createBaseIntegrationsServiceListAvailableRequest();
    return message;
  },
};

function createBaseIntegrationsServiceListAvailableResponse(): IntegrationsServiceListAvailableResponse {
  return { result: [] };
}

export const IntegrationsServiceListAvailableResponse = {
  encode(message: IntegrationsServiceListAvailableResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      IntegrationAvailableItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceListAvailableResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceListAvailableResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result.push(IntegrationAvailableItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceListAvailableResponse {
    return {
      result: Array.isArray(object?.result) ? object.result.map((e: any) => IntegrationAvailableItem.fromJSON(e)) : [],
    };
  },

  toJSON(message: IntegrationsServiceListAvailableResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? IntegrationAvailableItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceListAvailableResponse>, I>>(
    base?: I,
  ): IntegrationsServiceListAvailableResponse {
    return IntegrationsServiceListAvailableResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceListAvailableResponse>, I>>(
    object: I,
  ): IntegrationsServiceListAvailableResponse {
    const message = createBaseIntegrationsServiceListAvailableResponse();
    message.result = object.result?.map((e) => IntegrationAvailableItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseIntegrationAvailableItem(): IntegrationAvailableItem {
  return { id: "", version: "", description: "", fanout: undefined };
}

export const IntegrationAvailableItem = {
  encode(message: IntegrationAvailableItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.version !== "") {
      writer.uint32(18).string(message.version);
    }
    if (message.description !== "") {
      writer.uint32(26).string(message.description);
    }
    if (message.fanout !== undefined) {
      PluginFanout.encode(message.fanout, writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationAvailableItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationAvailableItem();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.version = reader.string();
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          message.description = reader.string();
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.fanout = PluginFanout.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationAvailableItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      version: isSet(object.version) ? String(object.version) : "",
      description: isSet(object.description) ? String(object.description) : "",
      fanout: isSet(object.fanout) ? PluginFanout.fromJSON(object.fanout) : undefined,
    };
  },

  toJSON(message: IntegrationAvailableItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.version !== undefined && (obj.version = message.version);
    message.description !== undefined && (obj.description = message.description);
    message.fanout !== undefined && (obj.fanout = message.fanout ? PluginFanout.toJSON(message.fanout) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationAvailableItem>, I>>(base?: I): IntegrationAvailableItem {
    return IntegrationAvailableItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationAvailableItem>, I>>(object: I): IntegrationAvailableItem {
    const message = createBaseIntegrationAvailableItem();
    message.id = object.id ?? "";
    message.version = object.version ?? "";
    message.description = object.description ?? "";
    message.fanout = (object.fanout !== undefined && object.fanout !== null)
      ? PluginFanout.fromPartial(object.fanout)
      : undefined;
    return message;
  },
};

function createBasePluginFanout(): PluginFanout {
  return { registrationSchema: new Uint8Array(), attachmentSchema: new Uint8Array(), subscribedMaterials: [] };
}

export const PluginFanout = {
  encode(message: PluginFanout, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.registrationSchema.length !== 0) {
      writer.uint32(34).bytes(message.registrationSchema);
    }
    if (message.attachmentSchema.length !== 0) {
      writer.uint32(42).bytes(message.attachmentSchema);
    }
    for (const v of message.subscribedMaterials) {
      writer.uint32(50).string(v!);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PluginFanout {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePluginFanout();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 4:
          if (tag != 34) {
            break;
          }

          message.registrationSchema = reader.bytes();
          continue;
        case 5:
          if (tag != 42) {
            break;
          }

          message.attachmentSchema = reader.bytes();
          continue;
        case 6:
          if (tag != 50) {
            break;
          }

          message.subscribedMaterials.push(reader.string());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PluginFanout {
    return {
      registrationSchema: isSet(object.registrationSchema)
        ? bytesFromBase64(object.registrationSchema)
        : new Uint8Array(),
      attachmentSchema: isSet(object.attachmentSchema) ? bytesFromBase64(object.attachmentSchema) : new Uint8Array(),
      subscribedMaterials: Array.isArray(object?.subscribedMaterials)
        ? object.subscribedMaterials.map((e: any) => String(e))
        : [],
    };
  },

  toJSON(message: PluginFanout): unknown {
    const obj: any = {};
    message.registrationSchema !== undefined &&
      (obj.registrationSchema = base64FromBytes(
        message.registrationSchema !== undefined ? message.registrationSchema : new Uint8Array(),
      ));
    message.attachmentSchema !== undefined &&
      (obj.attachmentSchema = base64FromBytes(
        message.attachmentSchema !== undefined ? message.attachmentSchema : new Uint8Array(),
      ));
    if (message.subscribedMaterials) {
      obj.subscribedMaterials = message.subscribedMaterials.map((e) => e);
    } else {
      obj.subscribedMaterials = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<PluginFanout>, I>>(base?: I): PluginFanout {
    return PluginFanout.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PluginFanout>, I>>(object: I): PluginFanout {
    const message = createBasePluginFanout();
    message.registrationSchema = object.registrationSchema ?? new Uint8Array();
    message.attachmentSchema = object.attachmentSchema ?? new Uint8Array();
    message.subscribedMaterials = object.subscribedMaterials?.map((e) => e) || [];
    return message;
  },
};

function createBaseIntegrationsServiceListRegistrationsRequest(): IntegrationsServiceListRegistrationsRequest {
  return {};
}

export const IntegrationsServiceListRegistrationsRequest = {
  encode(_: IntegrationsServiceListRegistrationsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceListRegistrationsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceListRegistrationsRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): IntegrationsServiceListRegistrationsRequest {
    return {};
  },

  toJSON(_: IntegrationsServiceListRegistrationsRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceListRegistrationsRequest>, I>>(
    base?: I,
  ): IntegrationsServiceListRegistrationsRequest {
    return IntegrationsServiceListRegistrationsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceListRegistrationsRequest>, I>>(
    _: I,
  ): IntegrationsServiceListRegistrationsRequest {
    const message = createBaseIntegrationsServiceListRegistrationsRequest();
    return message;
  },
};

function createBaseIntegrationsServiceListRegistrationsResponse(): IntegrationsServiceListRegistrationsResponse {
  return { result: [] };
}

export const IntegrationsServiceListRegistrationsResponse = {
  encode(message: IntegrationsServiceListRegistrationsResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      RegisteredIntegrationItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceListRegistrationsResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceListRegistrationsResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result.push(RegisteredIntegrationItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceListRegistrationsResponse {
    return {
      result: Array.isArray(object?.result) ? object.result.map((e: any) => RegisteredIntegrationItem.fromJSON(e)) : [],
    };
  },

  toJSON(message: IntegrationsServiceListRegistrationsResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? RegisteredIntegrationItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceListRegistrationsResponse>, I>>(
    base?: I,
  ): IntegrationsServiceListRegistrationsResponse {
    return IntegrationsServiceListRegistrationsResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceListRegistrationsResponse>, I>>(
    object: I,
  ): IntegrationsServiceListRegistrationsResponse {
    const message = createBaseIntegrationsServiceListRegistrationsResponse();
    message.result = object.result?.map((e) => RegisteredIntegrationItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseIntegrationsServiceDescribeRegistrationRequest(): IntegrationsServiceDescribeRegistrationRequest {
  return { id: "" };
}

export const IntegrationsServiceDescribeRegistrationRequest = {
  encode(
    message: IntegrationsServiceDescribeRegistrationRequest,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDescribeRegistrationRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDescribeRegistrationRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceDescribeRegistrationRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: IntegrationsServiceDescribeRegistrationRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDescribeRegistrationRequest>, I>>(
    base?: I,
  ): IntegrationsServiceDescribeRegistrationRequest {
    return IntegrationsServiceDescribeRegistrationRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDescribeRegistrationRequest>, I>>(
    object: I,
  ): IntegrationsServiceDescribeRegistrationRequest {
    const message = createBaseIntegrationsServiceDescribeRegistrationRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseIntegrationsServiceDescribeRegistrationResponse(): IntegrationsServiceDescribeRegistrationResponse {
  return { result: undefined };
}

export const IntegrationsServiceDescribeRegistrationResponse = {
  encode(
    message: IntegrationsServiceDescribeRegistrationResponse,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.result !== undefined) {
      RegisteredIntegrationItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDescribeRegistrationResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDescribeRegistrationResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = RegisteredIntegrationItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceDescribeRegistrationResponse {
    return { result: isSet(object.result) ? RegisteredIntegrationItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: IntegrationsServiceDescribeRegistrationResponse): unknown {
    const obj: any = {};
    message.result !== undefined &&
      (obj.result = message.result ? RegisteredIntegrationItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDescribeRegistrationResponse>, I>>(
    base?: I,
  ): IntegrationsServiceDescribeRegistrationResponse {
    return IntegrationsServiceDescribeRegistrationResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDescribeRegistrationResponse>, I>>(
    object: I,
  ): IntegrationsServiceDescribeRegistrationResponse {
    const message = createBaseIntegrationsServiceDescribeRegistrationResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? RegisteredIntegrationItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseIntegrationsServiceDetachRequest(): IntegrationsServiceDetachRequest {
  return { id: "" };
}

export const IntegrationsServiceDetachRequest = {
  encode(message: IntegrationsServiceDetachRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDetachRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDetachRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceDetachRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: IntegrationsServiceDetachRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDetachRequest>, I>>(
    base?: I,
  ): IntegrationsServiceDetachRequest {
    return IntegrationsServiceDetachRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDetachRequest>, I>>(
    object: I,
  ): IntegrationsServiceDetachRequest {
    const message = createBaseIntegrationsServiceDetachRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseIntegrationsServiceDetachResponse(): IntegrationsServiceDetachResponse {
  return {};
}

export const IntegrationsServiceDetachResponse = {
  encode(_: IntegrationsServiceDetachResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDetachResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDetachResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): IntegrationsServiceDetachResponse {
    return {};
  },

  toJSON(_: IntegrationsServiceDetachResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDetachResponse>, I>>(
    base?: I,
  ): IntegrationsServiceDetachResponse {
    return IntegrationsServiceDetachResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDetachResponse>, I>>(
    _: I,
  ): IntegrationsServiceDetachResponse {
    const message = createBaseIntegrationsServiceDetachResponse();
    return message;
  },
};

function createBaseListAttachmentsRequest(): ListAttachmentsRequest {
  return { workflowId: "" };
}

export const ListAttachmentsRequest = {
  encode(message: ListAttachmentsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.workflowId !== "") {
      writer.uint32(10).string(message.workflowId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ListAttachmentsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseListAttachmentsRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.workflowId = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ListAttachmentsRequest {
    return { workflowId: isSet(object.workflowId) ? String(object.workflowId) : "" };
  },

  toJSON(message: ListAttachmentsRequest): unknown {
    const obj: any = {};
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    return obj;
  },

  create<I extends Exact<DeepPartial<ListAttachmentsRequest>, I>>(base?: I): ListAttachmentsRequest {
    return ListAttachmentsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ListAttachmentsRequest>, I>>(object: I): ListAttachmentsRequest {
    const message = createBaseListAttachmentsRequest();
    message.workflowId = object.workflowId ?? "";
    return message;
  },
};

function createBaseListAttachmentsResponse(): ListAttachmentsResponse {
  return { result: [] };
}

export const ListAttachmentsResponse = {
  encode(message: ListAttachmentsResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      IntegrationAttachmentItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ListAttachmentsResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseListAttachmentsResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result.push(IntegrationAttachmentItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ListAttachmentsResponse {
    return {
      result: Array.isArray(object?.result) ? object.result.map((e: any) => IntegrationAttachmentItem.fromJSON(e)) : [],
    };
  },

  toJSON(message: ListAttachmentsResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? IntegrationAttachmentItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<ListAttachmentsResponse>, I>>(base?: I): ListAttachmentsResponse {
    return ListAttachmentsResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ListAttachmentsResponse>, I>>(object: I): ListAttachmentsResponse {
    const message = createBaseListAttachmentsResponse();
    message.result = object.result?.map((e) => IntegrationAttachmentItem.fromPartial(e)) || [];
    return message;
  },
};

function createBaseRegisteredIntegrationItem(): RegisteredIntegrationItem {
  return { id: "", kind: "", description: "", createdAt: undefined, config: new Uint8Array() };
}

export const RegisteredIntegrationItem = {
  encode(message: RegisteredIntegrationItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.kind !== "") {
      writer.uint32(18).string(message.kind);
    }
    if (message.description !== "") {
      writer.uint32(34).string(message.description);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(26).fork()).ldelim();
    }
    if (message.config.length !== 0) {
      writer.uint32(42).bytes(message.config);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): RegisteredIntegrationItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseRegisteredIntegrationItem();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.kind = reader.string();
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.description = reader.string();
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 5:
          if (tag != 42) {
            break;
          }

          message.config = reader.bytes();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): RegisteredIntegrationItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
      description: isSet(object.description) ? String(object.description) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      config: isSet(object.config) ? bytesFromBase64(object.config) : new Uint8Array(),
    };
  },

  toJSON(message: RegisteredIntegrationItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.kind !== undefined && (obj.kind = message.kind);
    message.description !== undefined && (obj.description = message.description);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.config !== undefined &&
      (obj.config = base64FromBytes(message.config !== undefined ? message.config : new Uint8Array()));
    return obj;
  },

  create<I extends Exact<DeepPartial<RegisteredIntegrationItem>, I>>(base?: I): RegisteredIntegrationItem {
    return RegisteredIntegrationItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<RegisteredIntegrationItem>, I>>(object: I): RegisteredIntegrationItem {
    const message = createBaseRegisteredIntegrationItem();
    message.id = object.id ?? "";
    message.kind = object.kind ?? "";
    message.description = object.description ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.config = object.config ?? new Uint8Array();
    return message;
  },
};

function createBaseIntegrationAttachmentItem(): IntegrationAttachmentItem {
  return { id: "", createdAt: undefined, config: new Uint8Array(), integration: undefined, workflow: undefined };
}

export const IntegrationAttachmentItem = {
  encode(message: IntegrationAttachmentItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(18).fork()).ldelim();
    }
    if (message.config.length !== 0) {
      writer.uint32(26).bytes(message.config);
    }
    if (message.integration !== undefined) {
      RegisteredIntegrationItem.encode(message.integration, writer.uint32(34).fork()).ldelim();
    }
    if (message.workflow !== undefined) {
      WorkflowItem.encode(message.workflow, writer.uint32(42).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationAttachmentItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationAttachmentItem();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 3:
          if (tag != 26) {
            break;
          }

          message.config = reader.bytes();
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.integration = RegisteredIntegrationItem.decode(reader, reader.uint32());
          continue;
        case 5:
          if (tag != 42) {
            break;
          }

          message.workflow = WorkflowItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationAttachmentItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      config: isSet(object.config) ? bytesFromBase64(object.config) : new Uint8Array(),
      integration: isSet(object.integration) ? RegisteredIntegrationItem.fromJSON(object.integration) : undefined,
      workflow: isSet(object.workflow) ? WorkflowItem.fromJSON(object.workflow) : undefined,
    };
  },

  toJSON(message: IntegrationAttachmentItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.config !== undefined &&
      (obj.config = base64FromBytes(message.config !== undefined ? message.config : new Uint8Array()));
    message.integration !== undefined &&
      (obj.integration = message.integration ? RegisteredIntegrationItem.toJSON(message.integration) : undefined);
    message.workflow !== undefined &&
      (obj.workflow = message.workflow ? WorkflowItem.toJSON(message.workflow) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationAttachmentItem>, I>>(base?: I): IntegrationAttachmentItem {
    return IntegrationAttachmentItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationAttachmentItem>, I>>(object: I): IntegrationAttachmentItem {
    const message = createBaseIntegrationAttachmentItem();
    message.id = object.id ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.config = object.config ?? new Uint8Array();
    message.integration = (object.integration !== undefined && object.integration !== null)
      ? RegisteredIntegrationItem.fromPartial(object.integration)
      : undefined;
    message.workflow = (object.workflow !== undefined && object.workflow !== null)
      ? WorkflowItem.fromPartial(object.workflow)
      : undefined;
    return message;
  },
};

function createBaseIntegrationsServiceDeregisterRequest(): IntegrationsServiceDeregisterRequest {
  return { id: "" };
}

export const IntegrationsServiceDeregisterRequest = {
  encode(message: IntegrationsServiceDeregisterRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDeregisterRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDeregisterRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.id = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceDeregisterRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: IntegrationsServiceDeregisterRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDeregisterRequest>, I>>(
    base?: I,
  ): IntegrationsServiceDeregisterRequest {
    return IntegrationsServiceDeregisterRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDeregisterRequest>, I>>(
    object: I,
  ): IntegrationsServiceDeregisterRequest {
    const message = createBaseIntegrationsServiceDeregisterRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseIntegrationsServiceDeregisterResponse(): IntegrationsServiceDeregisterResponse {
  return {};
}

export const IntegrationsServiceDeregisterResponse = {
  encode(_: IntegrationsServiceDeregisterResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDeregisterResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDeregisterResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): IntegrationsServiceDeregisterResponse {
    return {};
  },

  toJSON(_: IntegrationsServiceDeregisterResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDeregisterResponse>, I>>(
    base?: I,
  ): IntegrationsServiceDeregisterResponse {
    return IntegrationsServiceDeregisterResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDeregisterResponse>, I>>(
    _: I,
  ): IntegrationsServiceDeregisterResponse {
    const message = createBaseIntegrationsServiceDeregisterResponse();
    return message;
  },
};

export interface IntegrationsService {
  /** Integrations available and loaded in the controlplane ready to be used during registration */
  ListAvailable(
    request: DeepPartial<IntegrationsServiceListAvailableRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceListAvailableResponse>;
  /**
   * Registration Related operations
   * Register a new integration in the organization
   */
  Register(
    request: DeepPartial<IntegrationsServiceRegisterRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceRegisterResponse>;
  /** Delete registered integrations */
  Deregister(
    request: DeepPartial<IntegrationsServiceDeregisterRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDeregisterResponse>;
  /** List registered integrations */
  ListRegistrations(
    request: DeepPartial<IntegrationsServiceListRegistrationsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceListRegistrationsResponse>;
  /** View registered integration */
  DescribeRegistration(
    request: DeepPartial<IntegrationsServiceDescribeRegistrationRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDescribeRegistrationResponse>;
  /**
   * Attachment Related operations
   * Attach an integration to a workflow
   */
  Attach(
    request: DeepPartial<IntegrationsServiceAttachRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceAttachResponse>;
  /** Detach integration from a workflow */
  Detach(
    request: DeepPartial<IntegrationsServiceDetachRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDetachResponse>;
  /** List attachments */
  ListAttachments(
    request: DeepPartial<ListAttachmentsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ListAttachmentsResponse>;
}

export class IntegrationsServiceClientImpl implements IntegrationsService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.ListAvailable = this.ListAvailable.bind(this);
    this.Register = this.Register.bind(this);
    this.Deregister = this.Deregister.bind(this);
    this.ListRegistrations = this.ListRegistrations.bind(this);
    this.DescribeRegistration = this.DescribeRegistration.bind(this);
    this.Attach = this.Attach.bind(this);
    this.Detach = this.Detach.bind(this);
    this.ListAttachments = this.ListAttachments.bind(this);
  }

  ListAvailable(
    request: DeepPartial<IntegrationsServiceListAvailableRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceListAvailableResponse> {
    return this.rpc.unary(
      IntegrationsServiceListAvailableDesc,
      IntegrationsServiceListAvailableRequest.fromPartial(request),
      metadata,
    );
  }

  Register(
    request: DeepPartial<IntegrationsServiceRegisterRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceRegisterResponse> {
    return this.rpc.unary(
      IntegrationsServiceRegisterDesc,
      IntegrationsServiceRegisterRequest.fromPartial(request),
      metadata,
    );
  }

  Deregister(
    request: DeepPartial<IntegrationsServiceDeregisterRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDeregisterResponse> {
    return this.rpc.unary(
      IntegrationsServiceDeregisterDesc,
      IntegrationsServiceDeregisterRequest.fromPartial(request),
      metadata,
    );
  }

  ListRegistrations(
    request: DeepPartial<IntegrationsServiceListRegistrationsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceListRegistrationsResponse> {
    return this.rpc.unary(
      IntegrationsServiceListRegistrationsDesc,
      IntegrationsServiceListRegistrationsRequest.fromPartial(request),
      metadata,
    );
  }

  DescribeRegistration(
    request: DeepPartial<IntegrationsServiceDescribeRegistrationRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDescribeRegistrationResponse> {
    return this.rpc.unary(
      IntegrationsServiceDescribeRegistrationDesc,
      IntegrationsServiceDescribeRegistrationRequest.fromPartial(request),
      metadata,
    );
  }

  Attach(
    request: DeepPartial<IntegrationsServiceAttachRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceAttachResponse> {
    return this.rpc.unary(
      IntegrationsServiceAttachDesc,
      IntegrationsServiceAttachRequest.fromPartial(request),
      metadata,
    );
  }

  Detach(
    request: DeepPartial<IntegrationsServiceDetachRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDetachResponse> {
    return this.rpc.unary(
      IntegrationsServiceDetachDesc,
      IntegrationsServiceDetachRequest.fromPartial(request),
      metadata,
    );
  }

  ListAttachments(
    request: DeepPartial<ListAttachmentsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ListAttachmentsResponse> {
    return this.rpc.unary(
      IntegrationsServiceListAttachmentsDesc,
      ListAttachmentsRequest.fromPartial(request),
      metadata,
    );
  }
}

export const IntegrationsServiceDesc = { serviceName: "controlplane.v1.IntegrationsService" };

export const IntegrationsServiceListAvailableDesc: UnaryMethodDefinitionish = {
  methodName: "ListAvailable",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceListAvailableRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceListAvailableResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceRegisterDesc: UnaryMethodDefinitionish = {
  methodName: "Register",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceRegisterRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceRegisterResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceDeregisterDesc: UnaryMethodDefinitionish = {
  methodName: "Deregister",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceDeregisterRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceDeregisterResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceListRegistrationsDesc: UnaryMethodDefinitionish = {
  methodName: "ListRegistrations",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceListRegistrationsRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceListRegistrationsResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceDescribeRegistrationDesc: UnaryMethodDefinitionish = {
  methodName: "DescribeRegistration",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceDescribeRegistrationRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceDescribeRegistrationResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceAttachDesc: UnaryMethodDefinitionish = {
  methodName: "Attach",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceAttachRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceAttachResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceDetachDesc: UnaryMethodDefinitionish = {
  methodName: "Detach",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceDetachRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceDetachResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceListAttachmentsDesc: UnaryMethodDefinitionish = {
  methodName: "ListAttachments",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ListAttachmentsRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ListAttachmentsResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

interface UnaryMethodDefinitionishR extends grpc.UnaryMethodDefinition<any, any> {
  requestStream: any;
  responseStream: any;
}

type UnaryMethodDefinitionish = UnaryMethodDefinitionishR;

interface Rpc {
  unary<T extends UnaryMethodDefinitionish>(
    methodDesc: T,
    request: any,
    metadata: grpc.Metadata | undefined,
  ): Promise<any>;
}

export class GrpcWebImpl {
  private host: string;
  private options: {
    transport?: grpc.TransportFactory;

    debug?: boolean;
    metadata?: grpc.Metadata;
    upStreamRetryCodes?: number[];
  };

  constructor(
    host: string,
    options: {
      transport?: grpc.TransportFactory;

      debug?: boolean;
      metadata?: grpc.Metadata;
      upStreamRetryCodes?: number[];
    },
  ) {
    this.host = host;
    this.options = options;
  }

  unary<T extends UnaryMethodDefinitionish>(
    methodDesc: T,
    _request: any,
    metadata: grpc.Metadata | undefined,
  ): Promise<any> {
    const request = { ..._request, ...methodDesc.requestType };
    const maybeCombinedMetadata = metadata && this.options.metadata
      ? new BrowserHeaders({ ...this.options?.metadata.headersMap, ...metadata?.headersMap })
      : metadata || this.options.metadata;
    return new Promise((resolve, reject) => {
      grpc.unary(methodDesc, {
        request,
        host: this.host,
        metadata: maybeCombinedMetadata,
        transport: this.options.transport,
        debug: this.options.debug,
        onEnd: function (response) {
          if (response.status === grpc.Code.OK) {
            resolve(response.message!.toObject());
          } else {
            const err = new GrpcWebError(response.statusMessage, response.status, response.trailers);
            reject(err);
          }
        },
      });
    });
  }
}

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
  let millis = t.seconds * 1_000;
  millis += t.nanos / 1_000_000;
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

export class GrpcWebError extends tsProtoGlobalThis.Error {
  constructor(message: string, public code: grpc.Code, public metadata: grpc.Metadata) {
    super(message);
  }
}
