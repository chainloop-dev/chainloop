/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";
import { WorkflowItem } from "./response_messages";

export const protobufPackage = "controlplane.v1";

export interface AddDependencyTrackRequest {
  config?: IntegrationConfig_DependencyTrack;
  apiKey: string;
}

export interface AddDependencyTrackResponse {
  result?: IntegrationItem;
}

export interface IntegrationsServiceListRequest {
}

export interface IntegrationsServiceListResponse {
  result: IntegrationItem[];
}

export interface IntegrationsServiceAttachRequest {
  workflowId: string;
  integrationId: string;
  config?: IntegrationAttachmentConfig;
}

export interface IntegrationsServiceAttachResponse {
  result?: IntegrationAttachmentItem;
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

export interface IntegrationItem {
  id: string;
  kind: string;
  createdAt?: Date;
  config?: IntegrationConfig;
}

export interface IntegrationAttachmentItem {
  id: string;
  createdAt?: Date;
  config?: IntegrationAttachmentConfig;
  integration?: IntegrationItem;
  workflow?: WorkflowItem;
}

/** Configuration used when a Integration is created in an organization */
export interface IntegrationConfig {
  dependencyTrack?: IntegrationConfig_DependencyTrack | undefined;
}

export interface IntegrationConfig_DependencyTrack {
  domain: string;
  /** Support the option to automatically create projects if requested */
  allowAutoCreate: boolean;
}

/** Configuration used when a Integration is attached to a Workflow */
export interface IntegrationAttachmentConfig {
  dependencyTrack?: IntegrationAttachmentConfig_DependencyTrack | undefined;
}

export interface IntegrationAttachmentConfig_DependencyTrack {
  /** The integration might either use a pre-configured projectID */
  projectId?:
    | string
    | undefined;
  /** name of the project ot be auto created */
  projectName?: string | undefined;
}

export interface IntegrationsServiceDeleteRequest {
  id: string;
}

export interface IntegrationsServiceDeleteResponse {
}

function createBaseAddDependencyTrackRequest(): AddDependencyTrackRequest {
  return { config: undefined, apiKey: "" };
}

export const AddDependencyTrackRequest = {
  encode(message: AddDependencyTrackRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.config !== undefined) {
      IntegrationConfig_DependencyTrack.encode(message.config, writer.uint32(10).fork()).ldelim();
    }
    if (message.apiKey !== "") {
      writer.uint32(18).string(message.apiKey);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AddDependencyTrackRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAddDependencyTrackRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.config = IntegrationConfig_DependencyTrack.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.apiKey = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AddDependencyTrackRequest {
    return {
      config: isSet(object.config) ? IntegrationConfig_DependencyTrack.fromJSON(object.config) : undefined,
      apiKey: isSet(object.apiKey) ? String(object.apiKey) : "",
    };
  },

  toJSON(message: AddDependencyTrackRequest): unknown {
    const obj: any = {};
    message.config !== undefined &&
      (obj.config = message.config ? IntegrationConfig_DependencyTrack.toJSON(message.config) : undefined);
    message.apiKey !== undefined && (obj.apiKey = message.apiKey);
    return obj;
  },

  create<I extends Exact<DeepPartial<AddDependencyTrackRequest>, I>>(base?: I): AddDependencyTrackRequest {
    return AddDependencyTrackRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AddDependencyTrackRequest>, I>>(object: I): AddDependencyTrackRequest {
    const message = createBaseAddDependencyTrackRequest();
    message.config = (object.config !== undefined && object.config !== null)
      ? IntegrationConfig_DependencyTrack.fromPartial(object.config)
      : undefined;
    message.apiKey = object.apiKey ?? "";
    return message;
  },
};

function createBaseAddDependencyTrackResponse(): AddDependencyTrackResponse {
  return { result: undefined };
}

export const AddDependencyTrackResponse = {
  encode(message: AddDependencyTrackResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.result !== undefined) {
      IntegrationItem.encode(message.result, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): AddDependencyTrackResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseAddDependencyTrackResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result = IntegrationItem.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): AddDependencyTrackResponse {
    return { result: isSet(object.result) ? IntegrationItem.fromJSON(object.result) : undefined };
  },

  toJSON(message: AddDependencyTrackResponse): unknown {
    const obj: any = {};
    message.result !== undefined && (obj.result = message.result ? IntegrationItem.toJSON(message.result) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<AddDependencyTrackResponse>, I>>(base?: I): AddDependencyTrackResponse {
    return AddDependencyTrackResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<AddDependencyTrackResponse>, I>>(object: I): AddDependencyTrackResponse {
    const message = createBaseAddDependencyTrackResponse();
    message.result = (object.result !== undefined && object.result !== null)
      ? IntegrationItem.fromPartial(object.result)
      : undefined;
    return message;
  },
};

function createBaseIntegrationsServiceListRequest(): IntegrationsServiceListRequest {
  return {};
}

export const IntegrationsServiceListRequest = {
  encode(_: IntegrationsServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceListRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceListRequest();
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

  fromJSON(_: any): IntegrationsServiceListRequest {
    return {};
  },

  toJSON(_: IntegrationsServiceListRequest): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceListRequest>, I>>(base?: I): IntegrationsServiceListRequest {
    return IntegrationsServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceListRequest>, I>>(_: I): IntegrationsServiceListRequest {
    const message = createBaseIntegrationsServiceListRequest();
    return message;
  },
};

function createBaseIntegrationsServiceListResponse(): IntegrationsServiceListResponse {
  return { result: [] };
}

export const IntegrationsServiceListResponse = {
  encode(message: IntegrationsServiceListResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.result) {
      IntegrationItem.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceListResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceListResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.result.push(IntegrationItem.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationsServiceListResponse {
    return { result: Array.isArray(object?.result) ? object.result.map((e: any) => IntegrationItem.fromJSON(e)) : [] };
  },

  toJSON(message: IntegrationsServiceListResponse): unknown {
    const obj: any = {};
    if (message.result) {
      obj.result = message.result.map((e) => e ? IntegrationItem.toJSON(e) : undefined);
    } else {
      obj.result = [];
    }
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceListResponse>, I>>(base?: I): IntegrationsServiceListResponse {
    return IntegrationsServiceListResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceListResponse>, I>>(
    object: I,
  ): IntegrationsServiceListResponse {
    const message = createBaseIntegrationsServiceListResponse();
    message.result = object.result?.map((e) => IntegrationItem.fromPartial(e)) || [];
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
      IntegrationAttachmentConfig.encode(message.config, writer.uint32(26).fork()).ldelim();
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
        case 3:
          if (tag != 26) {
            break;
          }

          message.config = IntegrationAttachmentConfig.decode(reader, reader.uint32());
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
      config: isSet(object.config) ? IntegrationAttachmentConfig.fromJSON(object.config) : undefined,
    };
  },

  toJSON(message: IntegrationsServiceAttachRequest): unknown {
    const obj: any = {};
    message.workflowId !== undefined && (obj.workflowId = message.workflowId);
    message.integrationId !== undefined && (obj.integrationId = message.integrationId);
    message.config !== undefined &&
      (obj.config = message.config ? IntegrationAttachmentConfig.toJSON(message.config) : undefined);
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
    message.config = (object.config !== undefined && object.config !== null)
      ? IntegrationAttachmentConfig.fromPartial(object.config)
      : undefined;
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

function createBaseIntegrationItem(): IntegrationItem {
  return { id: "", kind: "", createdAt: undefined, config: undefined };
}

export const IntegrationItem = {
  encode(message: IntegrationItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.kind !== "") {
      writer.uint32(18).string(message.kind);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(26).fork()).ldelim();
    }
    if (message.config !== undefined) {
      IntegrationConfig.encode(message.config, writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationItem {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationItem();
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
        case 3:
          if (tag != 26) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.config = IntegrationConfig.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationItem {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      kind: isSet(object.kind) ? String(object.kind) : "",
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      config: isSet(object.config) ? IntegrationConfig.fromJSON(object.config) : undefined,
    };
  },

  toJSON(message: IntegrationItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.kind !== undefined && (obj.kind = message.kind);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.config !== undefined &&
      (obj.config = message.config ? IntegrationConfig.toJSON(message.config) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationItem>, I>>(base?: I): IntegrationItem {
    return IntegrationItem.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationItem>, I>>(object: I): IntegrationItem {
    const message = createBaseIntegrationItem();
    message.id = object.id ?? "";
    message.kind = object.kind ?? "";
    message.createdAt = object.createdAt ?? undefined;
    message.config = (object.config !== undefined && object.config !== null)
      ? IntegrationConfig.fromPartial(object.config)
      : undefined;
    return message;
  },
};

function createBaseIntegrationAttachmentItem(): IntegrationAttachmentItem {
  return { id: "", createdAt: undefined, config: undefined, integration: undefined, workflow: undefined };
}

export const IntegrationAttachmentItem = {
  encode(message: IntegrationAttachmentItem, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(18).fork()).ldelim();
    }
    if (message.config !== undefined) {
      IntegrationAttachmentConfig.encode(message.config, writer.uint32(26).fork()).ldelim();
    }
    if (message.integration !== undefined) {
      IntegrationItem.encode(message.integration, writer.uint32(34).fork()).ldelim();
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

          message.config = IntegrationAttachmentConfig.decode(reader, reader.uint32());
          continue;
        case 4:
          if (tag != 34) {
            break;
          }

          message.integration = IntegrationItem.decode(reader, reader.uint32());
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
      config: isSet(object.config) ? IntegrationAttachmentConfig.fromJSON(object.config) : undefined,
      integration: isSet(object.integration) ? IntegrationItem.fromJSON(object.integration) : undefined,
      workflow: isSet(object.workflow) ? WorkflowItem.fromJSON(object.workflow) : undefined,
    };
  },

  toJSON(message: IntegrationAttachmentItem): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.config !== undefined &&
      (obj.config = message.config ? IntegrationAttachmentConfig.toJSON(message.config) : undefined);
    message.integration !== undefined &&
      (obj.integration = message.integration ? IntegrationItem.toJSON(message.integration) : undefined);
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
    message.config = (object.config !== undefined && object.config !== null)
      ? IntegrationAttachmentConfig.fromPartial(object.config)
      : undefined;
    message.integration = (object.integration !== undefined && object.integration !== null)
      ? IntegrationItem.fromPartial(object.integration)
      : undefined;
    message.workflow = (object.workflow !== undefined && object.workflow !== null)
      ? WorkflowItem.fromPartial(object.workflow)
      : undefined;
    return message;
  },
};

function createBaseIntegrationConfig(): IntegrationConfig {
  return { dependencyTrack: undefined };
}

export const IntegrationConfig = {
  encode(message: IntegrationConfig, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.dependencyTrack !== undefined) {
      IntegrationConfig_DependencyTrack.encode(message.dependencyTrack, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationConfig {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationConfig();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.dependencyTrack = IntegrationConfig_DependencyTrack.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationConfig {
    return {
      dependencyTrack: isSet(object.dependencyTrack)
        ? IntegrationConfig_DependencyTrack.fromJSON(object.dependencyTrack)
        : undefined,
    };
  },

  toJSON(message: IntegrationConfig): unknown {
    const obj: any = {};
    message.dependencyTrack !== undefined && (obj.dependencyTrack = message.dependencyTrack
      ? IntegrationConfig_DependencyTrack.toJSON(message.dependencyTrack)
      : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationConfig>, I>>(base?: I): IntegrationConfig {
    return IntegrationConfig.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationConfig>, I>>(object: I): IntegrationConfig {
    const message = createBaseIntegrationConfig();
    message.dependencyTrack = (object.dependencyTrack !== undefined && object.dependencyTrack !== null)
      ? IntegrationConfig_DependencyTrack.fromPartial(object.dependencyTrack)
      : undefined;
    return message;
  },
};

function createBaseIntegrationConfig_DependencyTrack(): IntegrationConfig_DependencyTrack {
  return { domain: "", allowAutoCreate: false };
}

export const IntegrationConfig_DependencyTrack = {
  encode(message: IntegrationConfig_DependencyTrack, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.domain !== "") {
      writer.uint32(10).string(message.domain);
    }
    if (message.allowAutoCreate === true) {
      writer.uint32(16).bool(message.allowAutoCreate);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationConfig_DependencyTrack {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationConfig_DependencyTrack();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.domain = reader.string();
          continue;
        case 2:
          if (tag != 16) {
            break;
          }

          message.allowAutoCreate = reader.bool();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationConfig_DependencyTrack {
    return {
      domain: isSet(object.domain) ? String(object.domain) : "",
      allowAutoCreate: isSet(object.allowAutoCreate) ? Boolean(object.allowAutoCreate) : false,
    };
  },

  toJSON(message: IntegrationConfig_DependencyTrack): unknown {
    const obj: any = {};
    message.domain !== undefined && (obj.domain = message.domain);
    message.allowAutoCreate !== undefined && (obj.allowAutoCreate = message.allowAutoCreate);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationConfig_DependencyTrack>, I>>(
    base?: I,
  ): IntegrationConfig_DependencyTrack {
    return IntegrationConfig_DependencyTrack.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationConfig_DependencyTrack>, I>>(
    object: I,
  ): IntegrationConfig_DependencyTrack {
    const message = createBaseIntegrationConfig_DependencyTrack();
    message.domain = object.domain ?? "";
    message.allowAutoCreate = object.allowAutoCreate ?? false;
    return message;
  },
};

function createBaseIntegrationAttachmentConfig(): IntegrationAttachmentConfig {
  return { dependencyTrack: undefined };
}

export const IntegrationAttachmentConfig = {
  encode(message: IntegrationAttachmentConfig, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.dependencyTrack !== undefined) {
      IntegrationAttachmentConfig_DependencyTrack.encode(message.dependencyTrack, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationAttachmentConfig {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationAttachmentConfig();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.dependencyTrack = IntegrationAttachmentConfig_DependencyTrack.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationAttachmentConfig {
    return {
      dependencyTrack: isSet(object.dependencyTrack)
        ? IntegrationAttachmentConfig_DependencyTrack.fromJSON(object.dependencyTrack)
        : undefined,
    };
  },

  toJSON(message: IntegrationAttachmentConfig): unknown {
    const obj: any = {};
    message.dependencyTrack !== undefined && (obj.dependencyTrack = message.dependencyTrack
      ? IntegrationAttachmentConfig_DependencyTrack.toJSON(message.dependencyTrack)
      : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationAttachmentConfig>, I>>(base?: I): IntegrationAttachmentConfig {
    return IntegrationAttachmentConfig.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationAttachmentConfig>, I>>(object: I): IntegrationAttachmentConfig {
    const message = createBaseIntegrationAttachmentConfig();
    message.dependencyTrack = (object.dependencyTrack !== undefined && object.dependencyTrack !== null)
      ? IntegrationAttachmentConfig_DependencyTrack.fromPartial(object.dependencyTrack)
      : undefined;
    return message;
  },
};

function createBaseIntegrationAttachmentConfig_DependencyTrack(): IntegrationAttachmentConfig_DependencyTrack {
  return { projectId: undefined, projectName: undefined };
}

export const IntegrationAttachmentConfig_DependencyTrack = {
  encode(message: IntegrationAttachmentConfig_DependencyTrack, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.projectId !== undefined) {
      writer.uint32(10).string(message.projectId);
    }
    if (message.projectName !== undefined) {
      writer.uint32(18).string(message.projectName);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationAttachmentConfig_DependencyTrack {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationAttachmentConfig_DependencyTrack();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag != 10) {
            break;
          }

          message.projectId = reader.string();
          continue;
        case 2:
          if (tag != 18) {
            break;
          }

          message.projectName = reader.string();
          continue;
      }
      if ((tag & 7) == 4 || tag == 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): IntegrationAttachmentConfig_DependencyTrack {
    return {
      projectId: isSet(object.projectId) ? String(object.projectId) : undefined,
      projectName: isSet(object.projectName) ? String(object.projectName) : undefined,
    };
  },

  toJSON(message: IntegrationAttachmentConfig_DependencyTrack): unknown {
    const obj: any = {};
    message.projectId !== undefined && (obj.projectId = message.projectId);
    message.projectName !== undefined && (obj.projectName = message.projectName);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationAttachmentConfig_DependencyTrack>, I>>(
    base?: I,
  ): IntegrationAttachmentConfig_DependencyTrack {
    return IntegrationAttachmentConfig_DependencyTrack.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationAttachmentConfig_DependencyTrack>, I>>(
    object: I,
  ): IntegrationAttachmentConfig_DependencyTrack {
    const message = createBaseIntegrationAttachmentConfig_DependencyTrack();
    message.projectId = object.projectId ?? undefined;
    message.projectName = object.projectName ?? undefined;
    return message;
  },
};

function createBaseIntegrationsServiceDeleteRequest(): IntegrationsServiceDeleteRequest {
  return { id: "" };
}

export const IntegrationsServiceDeleteRequest = {
  encode(message: IntegrationsServiceDeleteRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDeleteRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDeleteRequest();
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

  fromJSON(object: any): IntegrationsServiceDeleteRequest {
    return { id: isSet(object.id) ? String(object.id) : "" };
  },

  toJSON(message: IntegrationsServiceDeleteRequest): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDeleteRequest>, I>>(
    base?: I,
  ): IntegrationsServiceDeleteRequest {
    return IntegrationsServiceDeleteRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDeleteRequest>, I>>(
    object: I,
  ): IntegrationsServiceDeleteRequest {
    const message = createBaseIntegrationsServiceDeleteRequest();
    message.id = object.id ?? "";
    return message;
  },
};

function createBaseIntegrationsServiceDeleteResponse(): IntegrationsServiceDeleteResponse {
  return {};
}

export const IntegrationsServiceDeleteResponse = {
  encode(_: IntegrationsServiceDeleteResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): IntegrationsServiceDeleteResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseIntegrationsServiceDeleteResponse();
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

  fromJSON(_: any): IntegrationsServiceDeleteResponse {
    return {};
  },

  toJSON(_: IntegrationsServiceDeleteResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<IntegrationsServiceDeleteResponse>, I>>(
    base?: I,
  ): IntegrationsServiceDeleteResponse {
    return IntegrationsServiceDeleteResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<IntegrationsServiceDeleteResponse>, I>>(
    _: I,
  ): IntegrationsServiceDeleteResponse {
    const message = createBaseIntegrationsServiceDeleteResponse();
    return message;
  },
};

export interface IntegrationsService {
  /** ORG related CRUD */
  AddDependencyTrack(
    request: DeepPartial<AddDependencyTrackRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AddDependencyTrackResponse>;
  List(
    request: DeepPartial<IntegrationsServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceListResponse>;
  Delete(
    request: DeepPartial<IntegrationsServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDeleteResponse>;
  /**
   * Workflow Related operations
   * Attach to a workflow
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
  ListAttachments(
    request: DeepPartial<ListAttachmentsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ListAttachmentsResponse>;
}

export class IntegrationsServiceClientImpl implements IntegrationsService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.AddDependencyTrack = this.AddDependencyTrack.bind(this);
    this.List = this.List.bind(this);
    this.Delete = this.Delete.bind(this);
    this.Attach = this.Attach.bind(this);
    this.Detach = this.Detach.bind(this);
    this.ListAttachments = this.ListAttachments.bind(this);
  }

  AddDependencyTrack(
    request: DeepPartial<AddDependencyTrackRequest>,
    metadata?: grpc.Metadata,
  ): Promise<AddDependencyTrackResponse> {
    return this.rpc.unary(
      IntegrationsServiceAddDependencyTrackDesc,
      AddDependencyTrackRequest.fromPartial(request),
      metadata,
    );
  }

  List(
    request: DeepPartial<IntegrationsServiceListRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceListResponse> {
    return this.rpc.unary(IntegrationsServiceListDesc, IntegrationsServiceListRequest.fromPartial(request), metadata);
  }

  Delete(
    request: DeepPartial<IntegrationsServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<IntegrationsServiceDeleteResponse> {
    return this.rpc.unary(
      IntegrationsServiceDeleteDesc,
      IntegrationsServiceDeleteRequest.fromPartial(request),
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

export const IntegrationsServiceAddDependencyTrackDesc: UnaryMethodDefinitionish = {
  methodName: "AddDependencyTrack",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return AddDependencyTrackRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = AddDependencyTrackResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceListDesc: UnaryMethodDefinitionish = {
  methodName: "List",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceListRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceListResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const IntegrationsServiceDeleteDesc: UnaryMethodDefinitionish = {
  methodName: "Delete",
  service: IntegrationsServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return IntegrationsServiceDeleteRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = IntegrationsServiceDeleteResponse.decode(data);
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

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}

export class GrpcWebError extends tsProtoGlobalThis.Error {
  constructor(message: string, public code: grpc.Code, public metadata: grpc.Metadata) {
    super(message);
  }
}
