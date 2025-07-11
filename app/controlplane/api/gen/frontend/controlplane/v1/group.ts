/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";
import { OffsetPaginationRequest, OffsetPaginationResponse } from "./pagination";
import { User } from "./response_messages";
import { Group, IdentityReference } from "./shared_message";

export const protobufPackage = "controlplane.v1";

/** GroupServiceCreateRequest contains the information needed to create a new group */
export interface GroupServiceCreateRequest {
  /** Name of the group to create */
  name: string;
  /** Description providing additional information about the group */
  description: string;
}

/** GroupServiceCreateResponse contains the newly created group */
export interface GroupServiceCreateResponse {
  /** The created group with all its attributes */
  group?: Group;
}

/** GroupServiceGetRequest contains the identifier for the group to retrieve */
export interface GroupServiceGetRequest {
  /** IdentityReference is used to specify the group by either its ID or name */
  groupReference?: IdentityReference;
}

/** GroupServiceGetResponse contains the requested group information */
export interface GroupServiceGetResponse {
  /** The requested group with all its attributes */
  group?: Group;
}

/** GroupServiceListsRequest contains parameters for filtering and paginating group results */
export interface GroupServiceListRequest {
  /** Optional filter to search by group name */
  name?:
    | string
    | undefined;
  /** Optional filter to search by group description */
  description?:
    | string
    | undefined;
  /** Optional filter to search by member email address */
  memberEmail?:
    | string
    | undefined;
  /** Pagination parameters to limit and offset results */
  pagination?: OffsetPaginationRequest;
}

/** GroupServiceListsResponse contains a paginated list of groups */
export interface GroupServiceListResponse {
  /** List of groups matching the request criteria */
  groups: Group[];
  /** Pagination information for the response */
  pagination?: OffsetPaginationResponse;
}

/** GroupServiceUpdateRequest contains the fields that can be updated for a group */
export interface GroupServiceUpdateRequest {
  /** IdentityReference is used to specify the group by either its ID or name */
  groupReference?: IdentityReference;
  /** New name for the group (if provided) */
  newName?:
    | string
    | undefined;
  /** New description for the group (if provided) */
  newDescription?: string | undefined;
}

/** GroupServiceUpdateResponse contains the updated group information */
export interface GroupServiceUpdateResponse {
  /** The updated group with all its attributes */
  group?: Group;
}

/** GroupServiceDeleteRequest contains the identifier for the group to delete */
export interface GroupServiceDeleteRequest {
  /** IdentityReference is used to specify the group by either its ID or name */
  groupReference?: IdentityReference;
}

/** GroupServiceDeleteResponse is returned upon successful deletion of a group */
export interface GroupServiceDeleteResponse {
}

export interface GroupServiceListMembersResponse {
  /** List of members in the group */
  members: GroupMember[];
  /** Pagination information for the response */
  pagination?: OffsetPaginationResponse;
}

/** GroupServiceListMembersRequest contains the identifier for the group whose members are to be listed */
export interface GroupServiceListMembersRequest {
  /** IdentityReference is used to specify the group by either its ID or name */
  groupReference?: IdentityReference;
  /** Optional filter to search only by maintainers or not */
  maintainers?:
    | boolean
    | undefined;
  /** Optional filter to search by member email address */
  memberEmail?:
    | string
    | undefined;
  /** Pagination parameters to limit and offset results */
  pagination?: OffsetPaginationRequest;
}

/** GroupServiceAddMemberRequest contains the information needed to add a user to a group */
export interface GroupServiceAddMemberRequest {
  /** IdentityReference is used to specify the group by either its ID or name */
  groupReference?: IdentityReference;
  /** The user to add to the group */
  userEmail: string;
  /** Indicates whether the user should have maintainer (admin) privileges in the group */
  isMaintainer: boolean;
}

/** GroupServiceAddMemberResponse contains the information about the group member that was added */
export interface GroupServiceAddMemberResponse {
}

/** GroupServiceRemoveMemberRequest contains the information needed to remove a user from a group */
export interface GroupServiceRemoveMemberRequest {
  /** IdentityReference is used to specify the group by either its ID or name */
  groupReference?: IdentityReference;
  /** The user to remove from the group */
  userEmail: string;
}

/** GroupServiceRemoveMemberResponse is returned upon successful removal of a user from a group */
export interface GroupServiceRemoveMemberResponse {
}

export interface GroupServiceListPendingInvitationsRequest {
  /** IdentityReference is used to specify the group by either its ID or name */
  groupReference?: IdentityReference;
  /** Pagination parameters to limit and offset results */
  pagination?: OffsetPaginationRequest;
}

/** GroupServiceListPendingInvitationsResponse contains a list of pending invitations for a group */
export interface GroupServiceListPendingInvitationsResponse {
  /** List of pending invitations for the group */
  invitations: PendingGroupInvitation[];
  /** Pagination information for the response */
  pagination?: OffsetPaginationResponse;
}

/** PendingInvitation represents an invitation to join a group that has not yet been accepted */
export interface PendingGroupInvitation {
  /** The email address of the user invited to the group */
  userEmail: string;
  /** The user who sent the invitation */
  invitedBy?:
    | User
    | undefined;
  /** Timestamp when the invitation was created */
  createdAt?: Date;
  /** Unique identifier for the invitation */
  invitationId: string;
}

/** GroupMember represents a user's membership within a group with their role information */
export interface GroupMember {
  /** The user who is a member of the group */
  user?: User;
  /** Indicates whether the user has maintainer (admin) privileges in the group */
  isMaintainer: boolean;
  /** Timestamp when the group membership was created */
  createdAt?: Date;
  /** Timestamp when the group membership was last modified */
  updatedAt?: Date;
}

/** GroupServiceUpdateMemberMaintainerStatusRequest contains the information needed to update a member's maintainer status */
export interface GroupServiceUpdateMemberMaintainerStatusRequest {
  /** IdentityReference is used to specify the group by either its ID or name */
  groupReference?: IdentityReference;
  /** The user whose maintainer status is to be updated */
  userId: string;
  /** The new maintainer status for the user */
  isMaintainer: boolean;
}

/** GroupServiceUpdateMemberMaintainerStatusResponse is returned upon successful update of a member's maintainer status */
export interface GroupServiceUpdateMemberMaintainerStatusResponse {
}

/** GroupServiceListProjectsRequest contains parameters for filtering and paginating project results for a group */
export interface GroupServiceListProjectsRequest {
  /** IdentityReference is used to specify the group by either its ID or name */
  groupReference?: IdentityReference;
  /** Pagination parameters to limit and offset results */
  pagination?: OffsetPaginationRequest;
}

/** GroupServiceListProjectsResponse contains a paginated list of projects for a group */
export interface GroupServiceListProjectsResponse {
  /** List of projects memberships matching the request criteria */
  projects: ProjectInfo[];
  /** Pagination information for the response */
  pagination?: OffsetPaginationResponse;
}

/** ProjectInfo represents detailed information about a project that a group is a member of */
export interface ProjectInfo {
  /** Unique identifier of the project */
  id: string;
  /** Name of the project */
  name: string;
  /** Description of the project */
  description: string;
  /** Role of the group in the project (admin or viewer) */
  role: string;
  /** The latest version ID of the project, if available */
  latestVersionId?:
    | string
    | undefined;
  /** Timestamp when the project was created */
  createdAt?: Date;
}

function createBaseGroupServiceCreateRequest(): GroupServiceCreateRequest {
  return { name: "", description: "" };
}

export const GroupServiceCreateRequest = {
  encode(message: GroupServiceCreateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== "") {
      writer.uint32(10).string(message.name);
    }
    if (message.description !== "") {
      writer.uint32(18).string(message.description);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceCreateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceCreateRequest();
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

          message.description = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceCreateRequest {
    return {
      name: isSet(object.name) ? String(object.name) : "",
      description: isSet(object.description) ? String(object.description) : "",
    };
  },

  toJSON(message: GroupServiceCreateRequest): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.description !== undefined && (obj.description = message.description);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceCreateRequest>, I>>(base?: I): GroupServiceCreateRequest {
    return GroupServiceCreateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceCreateRequest>, I>>(object: I): GroupServiceCreateRequest {
    const message = createBaseGroupServiceCreateRequest();
    message.name = object.name ?? "";
    message.description = object.description ?? "";
    return message;
  },
};

function createBaseGroupServiceCreateResponse(): GroupServiceCreateResponse {
  return { group: undefined };
}

export const GroupServiceCreateResponse = {
  encode(message: GroupServiceCreateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.group !== undefined) {
      Group.encode(message.group, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceCreateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceCreateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.group = Group.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceCreateResponse {
    return { group: isSet(object.group) ? Group.fromJSON(object.group) : undefined };
  },

  toJSON(message: GroupServiceCreateResponse): unknown {
    const obj: any = {};
    message.group !== undefined && (obj.group = message.group ? Group.toJSON(message.group) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceCreateResponse>, I>>(base?: I): GroupServiceCreateResponse {
    return GroupServiceCreateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceCreateResponse>, I>>(object: I): GroupServiceCreateResponse {
    const message = createBaseGroupServiceCreateResponse();
    message.group = (object.group !== undefined && object.group !== null) ? Group.fromPartial(object.group) : undefined;
    return message;
  },
};

function createBaseGroupServiceGetRequest(): GroupServiceGetRequest {
  return { groupReference: undefined };
}

export const GroupServiceGetRequest = {
  encode(message: GroupServiceGetRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.groupReference !== undefined) {
      IdentityReference.encode(message.groupReference, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceGetRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceGetRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.groupReference = IdentityReference.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceGetRequest {
    return {
      groupReference: isSet(object.groupReference) ? IdentityReference.fromJSON(object.groupReference) : undefined,
    };
  },

  toJSON(message: GroupServiceGetRequest): unknown {
    const obj: any = {};
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? IdentityReference.toJSON(message.groupReference) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceGetRequest>, I>>(base?: I): GroupServiceGetRequest {
    return GroupServiceGetRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceGetRequest>, I>>(object: I): GroupServiceGetRequest {
    const message = createBaseGroupServiceGetRequest();
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? IdentityReference.fromPartial(object.groupReference)
      : undefined;
    return message;
  },
};

function createBaseGroupServiceGetResponse(): GroupServiceGetResponse {
  return { group: undefined };
}

export const GroupServiceGetResponse = {
  encode(message: GroupServiceGetResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.group !== undefined) {
      Group.encode(message.group, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceGetResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceGetResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.group = Group.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceGetResponse {
    return { group: isSet(object.group) ? Group.fromJSON(object.group) : undefined };
  },

  toJSON(message: GroupServiceGetResponse): unknown {
    const obj: any = {};
    message.group !== undefined && (obj.group = message.group ? Group.toJSON(message.group) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceGetResponse>, I>>(base?: I): GroupServiceGetResponse {
    return GroupServiceGetResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceGetResponse>, I>>(object: I): GroupServiceGetResponse {
    const message = createBaseGroupServiceGetResponse();
    message.group = (object.group !== undefined && object.group !== null) ? Group.fromPartial(object.group) : undefined;
    return message;
  },
};

function createBaseGroupServiceListRequest(): GroupServiceListRequest {
  return { name: undefined, description: undefined, memberEmail: undefined, pagination: undefined };
}

export const GroupServiceListRequest = {
  encode(message: GroupServiceListRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.name !== undefined) {
      writer.uint32(10).string(message.name);
    }
    if (message.description !== undefined) {
      writer.uint32(18).string(message.description);
    }
    if (message.memberEmail !== undefined) {
      writer.uint32(26).string(message.memberEmail);
    }
    if (message.pagination !== undefined) {
      OffsetPaginationRequest.encode(message.pagination, writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceListRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceListRequest();
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

          message.description = reader.string();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.memberEmail = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.pagination = OffsetPaginationRequest.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceListRequest {
    return {
      name: isSet(object.name) ? String(object.name) : undefined,
      description: isSet(object.description) ? String(object.description) : undefined,
      memberEmail: isSet(object.memberEmail) ? String(object.memberEmail) : undefined,
      pagination: isSet(object.pagination) ? OffsetPaginationRequest.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: GroupServiceListRequest): unknown {
    const obj: any = {};
    message.name !== undefined && (obj.name = message.name);
    message.description !== undefined && (obj.description = message.description);
    message.memberEmail !== undefined && (obj.memberEmail = message.memberEmail);
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationRequest.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceListRequest>, I>>(base?: I): GroupServiceListRequest {
    return GroupServiceListRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceListRequest>, I>>(object: I): GroupServiceListRequest {
    const message = createBaseGroupServiceListRequest();
    message.name = object.name ?? undefined;
    message.description = object.description ?? undefined;
    message.memberEmail = object.memberEmail ?? undefined;
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationRequest.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseGroupServiceListResponse(): GroupServiceListResponse {
  return { groups: [], pagination: undefined };
}

export const GroupServiceListResponse = {
  encode(message: GroupServiceListResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.groups) {
      Group.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      OffsetPaginationResponse.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceListResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceListResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.groups.push(Group.decode(reader, reader.uint32()));
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.pagination = OffsetPaginationResponse.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceListResponse {
    return {
      groups: Array.isArray(object?.groups) ? object.groups.map((e: any) => Group.fromJSON(e)) : [],
      pagination: isSet(object.pagination) ? OffsetPaginationResponse.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: GroupServiceListResponse): unknown {
    const obj: any = {};
    if (message.groups) {
      obj.groups = message.groups.map((e) => e ? Group.toJSON(e) : undefined);
    } else {
      obj.groups = [];
    }
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationResponse.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceListResponse>, I>>(base?: I): GroupServiceListResponse {
    return GroupServiceListResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceListResponse>, I>>(object: I): GroupServiceListResponse {
    const message = createBaseGroupServiceListResponse();
    message.groups = object.groups?.map((e) => Group.fromPartial(e)) || [];
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationResponse.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseGroupServiceUpdateRequest(): GroupServiceUpdateRequest {
  return { groupReference: undefined, newName: undefined, newDescription: undefined };
}

export const GroupServiceUpdateRequest = {
  encode(message: GroupServiceUpdateRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.groupReference !== undefined) {
      IdentityReference.encode(message.groupReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.newName !== undefined) {
      writer.uint32(26).string(message.newName);
    }
    if (message.newDescription !== undefined) {
      writer.uint32(34).string(message.newDescription);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceUpdateRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceUpdateRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.groupReference = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.newName = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.newDescription = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceUpdateRequest {
    return {
      groupReference: isSet(object.groupReference) ? IdentityReference.fromJSON(object.groupReference) : undefined,
      newName: isSet(object.newName) ? String(object.newName) : undefined,
      newDescription: isSet(object.newDescription) ? String(object.newDescription) : undefined,
    };
  },

  toJSON(message: GroupServiceUpdateRequest): unknown {
    const obj: any = {};
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? IdentityReference.toJSON(message.groupReference) : undefined);
    message.newName !== undefined && (obj.newName = message.newName);
    message.newDescription !== undefined && (obj.newDescription = message.newDescription);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceUpdateRequest>, I>>(base?: I): GroupServiceUpdateRequest {
    return GroupServiceUpdateRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceUpdateRequest>, I>>(object: I): GroupServiceUpdateRequest {
    const message = createBaseGroupServiceUpdateRequest();
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? IdentityReference.fromPartial(object.groupReference)
      : undefined;
    message.newName = object.newName ?? undefined;
    message.newDescription = object.newDescription ?? undefined;
    return message;
  },
};

function createBaseGroupServiceUpdateResponse(): GroupServiceUpdateResponse {
  return { group: undefined };
}

export const GroupServiceUpdateResponse = {
  encode(message: GroupServiceUpdateResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.group !== undefined) {
      Group.encode(message.group, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceUpdateResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceUpdateResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.group = Group.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceUpdateResponse {
    return { group: isSet(object.group) ? Group.fromJSON(object.group) : undefined };
  },

  toJSON(message: GroupServiceUpdateResponse): unknown {
    const obj: any = {};
    message.group !== undefined && (obj.group = message.group ? Group.toJSON(message.group) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceUpdateResponse>, I>>(base?: I): GroupServiceUpdateResponse {
    return GroupServiceUpdateResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceUpdateResponse>, I>>(object: I): GroupServiceUpdateResponse {
    const message = createBaseGroupServiceUpdateResponse();
    message.group = (object.group !== undefined && object.group !== null) ? Group.fromPartial(object.group) : undefined;
    return message;
  },
};

function createBaseGroupServiceDeleteRequest(): GroupServiceDeleteRequest {
  return { groupReference: undefined };
}

export const GroupServiceDeleteRequest = {
  encode(message: GroupServiceDeleteRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.groupReference !== undefined) {
      IdentityReference.encode(message.groupReference, writer.uint32(10).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceDeleteRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceDeleteRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.groupReference = IdentityReference.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceDeleteRequest {
    return {
      groupReference: isSet(object.groupReference) ? IdentityReference.fromJSON(object.groupReference) : undefined,
    };
  },

  toJSON(message: GroupServiceDeleteRequest): unknown {
    const obj: any = {};
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? IdentityReference.toJSON(message.groupReference) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceDeleteRequest>, I>>(base?: I): GroupServiceDeleteRequest {
    return GroupServiceDeleteRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceDeleteRequest>, I>>(object: I): GroupServiceDeleteRequest {
    const message = createBaseGroupServiceDeleteRequest();
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? IdentityReference.fromPartial(object.groupReference)
      : undefined;
    return message;
  },
};

function createBaseGroupServiceDeleteResponse(): GroupServiceDeleteResponse {
  return {};
}

export const GroupServiceDeleteResponse = {
  encode(_: GroupServiceDeleteResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceDeleteResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceDeleteResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): GroupServiceDeleteResponse {
    return {};
  },

  toJSON(_: GroupServiceDeleteResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceDeleteResponse>, I>>(base?: I): GroupServiceDeleteResponse {
    return GroupServiceDeleteResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceDeleteResponse>, I>>(_: I): GroupServiceDeleteResponse {
    const message = createBaseGroupServiceDeleteResponse();
    return message;
  },
};

function createBaseGroupServiceListMembersResponse(): GroupServiceListMembersResponse {
  return { members: [], pagination: undefined };
}

export const GroupServiceListMembersResponse = {
  encode(message: GroupServiceListMembersResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.members) {
      GroupMember.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      OffsetPaginationResponse.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceListMembersResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceListMembersResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.members.push(GroupMember.decode(reader, reader.uint32()));
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.pagination = OffsetPaginationResponse.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceListMembersResponse {
    return {
      members: Array.isArray(object?.members) ? object.members.map((e: any) => GroupMember.fromJSON(e)) : [],
      pagination: isSet(object.pagination) ? OffsetPaginationResponse.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: GroupServiceListMembersResponse): unknown {
    const obj: any = {};
    if (message.members) {
      obj.members = message.members.map((e) => e ? GroupMember.toJSON(e) : undefined);
    } else {
      obj.members = [];
    }
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationResponse.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceListMembersResponse>, I>>(base?: I): GroupServiceListMembersResponse {
    return GroupServiceListMembersResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceListMembersResponse>, I>>(
    object: I,
  ): GroupServiceListMembersResponse {
    const message = createBaseGroupServiceListMembersResponse();
    message.members = object.members?.map((e) => GroupMember.fromPartial(e)) || [];
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationResponse.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseGroupServiceListMembersRequest(): GroupServiceListMembersRequest {
  return { groupReference: undefined, maintainers: undefined, memberEmail: undefined, pagination: undefined };
}

export const GroupServiceListMembersRequest = {
  encode(message: GroupServiceListMembersRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.groupReference !== undefined) {
      IdentityReference.encode(message.groupReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.maintainers !== undefined) {
      writer.uint32(24).bool(message.maintainers);
    }
    if (message.memberEmail !== undefined) {
      writer.uint32(34).string(message.memberEmail);
    }
    if (message.pagination !== undefined) {
      OffsetPaginationRequest.encode(message.pagination, writer.uint32(42).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceListMembersRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceListMembersRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.groupReference = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.maintainers = reader.bool();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.memberEmail = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.pagination = OffsetPaginationRequest.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceListMembersRequest {
    return {
      groupReference: isSet(object.groupReference) ? IdentityReference.fromJSON(object.groupReference) : undefined,
      maintainers: isSet(object.maintainers) ? Boolean(object.maintainers) : undefined,
      memberEmail: isSet(object.memberEmail) ? String(object.memberEmail) : undefined,
      pagination: isSet(object.pagination) ? OffsetPaginationRequest.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: GroupServiceListMembersRequest): unknown {
    const obj: any = {};
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? IdentityReference.toJSON(message.groupReference) : undefined);
    message.maintainers !== undefined && (obj.maintainers = message.maintainers);
    message.memberEmail !== undefined && (obj.memberEmail = message.memberEmail);
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationRequest.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceListMembersRequest>, I>>(base?: I): GroupServiceListMembersRequest {
    return GroupServiceListMembersRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceListMembersRequest>, I>>(
    object: I,
  ): GroupServiceListMembersRequest {
    const message = createBaseGroupServiceListMembersRequest();
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? IdentityReference.fromPartial(object.groupReference)
      : undefined;
    message.maintainers = object.maintainers ?? undefined;
    message.memberEmail = object.memberEmail ?? undefined;
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationRequest.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseGroupServiceAddMemberRequest(): GroupServiceAddMemberRequest {
  return { groupReference: undefined, userEmail: "", isMaintainer: false };
}

export const GroupServiceAddMemberRequest = {
  encode(message: GroupServiceAddMemberRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.groupReference !== undefined) {
      IdentityReference.encode(message.groupReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.userEmail !== "") {
      writer.uint32(26).string(message.userEmail);
    }
    if (message.isMaintainer === true) {
      writer.uint32(32).bool(message.isMaintainer);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceAddMemberRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceAddMemberRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.groupReference = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.userEmail = reader.string();
          continue;
        case 4:
          if (tag !== 32) {
            break;
          }

          message.isMaintainer = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceAddMemberRequest {
    return {
      groupReference: isSet(object.groupReference) ? IdentityReference.fromJSON(object.groupReference) : undefined,
      userEmail: isSet(object.userEmail) ? String(object.userEmail) : "",
      isMaintainer: isSet(object.isMaintainer) ? Boolean(object.isMaintainer) : false,
    };
  },

  toJSON(message: GroupServiceAddMemberRequest): unknown {
    const obj: any = {};
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? IdentityReference.toJSON(message.groupReference) : undefined);
    message.userEmail !== undefined && (obj.userEmail = message.userEmail);
    message.isMaintainer !== undefined && (obj.isMaintainer = message.isMaintainer);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceAddMemberRequest>, I>>(base?: I): GroupServiceAddMemberRequest {
    return GroupServiceAddMemberRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceAddMemberRequest>, I>>(object: I): GroupServiceAddMemberRequest {
    const message = createBaseGroupServiceAddMemberRequest();
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? IdentityReference.fromPartial(object.groupReference)
      : undefined;
    message.userEmail = object.userEmail ?? "";
    message.isMaintainer = object.isMaintainer ?? false;
    return message;
  },
};

function createBaseGroupServiceAddMemberResponse(): GroupServiceAddMemberResponse {
  return {};
}

export const GroupServiceAddMemberResponse = {
  encode(_: GroupServiceAddMemberResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceAddMemberResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceAddMemberResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): GroupServiceAddMemberResponse {
    return {};
  },

  toJSON(_: GroupServiceAddMemberResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceAddMemberResponse>, I>>(base?: I): GroupServiceAddMemberResponse {
    return GroupServiceAddMemberResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceAddMemberResponse>, I>>(_: I): GroupServiceAddMemberResponse {
    const message = createBaseGroupServiceAddMemberResponse();
    return message;
  },
};

function createBaseGroupServiceRemoveMemberRequest(): GroupServiceRemoveMemberRequest {
  return { groupReference: undefined, userEmail: "" };
}

export const GroupServiceRemoveMemberRequest = {
  encode(message: GroupServiceRemoveMemberRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.groupReference !== undefined) {
      IdentityReference.encode(message.groupReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.userEmail !== "") {
      writer.uint32(26).string(message.userEmail);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceRemoveMemberRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceRemoveMemberRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.groupReference = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.userEmail = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceRemoveMemberRequest {
    return {
      groupReference: isSet(object.groupReference) ? IdentityReference.fromJSON(object.groupReference) : undefined,
      userEmail: isSet(object.userEmail) ? String(object.userEmail) : "",
    };
  },

  toJSON(message: GroupServiceRemoveMemberRequest): unknown {
    const obj: any = {};
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? IdentityReference.toJSON(message.groupReference) : undefined);
    message.userEmail !== undefined && (obj.userEmail = message.userEmail);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceRemoveMemberRequest>, I>>(base?: I): GroupServiceRemoveMemberRequest {
    return GroupServiceRemoveMemberRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceRemoveMemberRequest>, I>>(
    object: I,
  ): GroupServiceRemoveMemberRequest {
    const message = createBaseGroupServiceRemoveMemberRequest();
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? IdentityReference.fromPartial(object.groupReference)
      : undefined;
    message.userEmail = object.userEmail ?? "";
    return message;
  },
};

function createBaseGroupServiceRemoveMemberResponse(): GroupServiceRemoveMemberResponse {
  return {};
}

export const GroupServiceRemoveMemberResponse = {
  encode(_: GroupServiceRemoveMemberResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceRemoveMemberResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceRemoveMemberResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): GroupServiceRemoveMemberResponse {
    return {};
  },

  toJSON(_: GroupServiceRemoveMemberResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceRemoveMemberResponse>, I>>(
    base?: I,
  ): GroupServiceRemoveMemberResponse {
    return GroupServiceRemoveMemberResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceRemoveMemberResponse>, I>>(
    _: I,
  ): GroupServiceRemoveMemberResponse {
    const message = createBaseGroupServiceRemoveMemberResponse();
    return message;
  },
};

function createBaseGroupServiceListPendingInvitationsRequest(): GroupServiceListPendingInvitationsRequest {
  return { groupReference: undefined, pagination: undefined };
}

export const GroupServiceListPendingInvitationsRequest = {
  encode(message: GroupServiceListPendingInvitationsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.groupReference !== undefined) {
      IdentityReference.encode(message.groupReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      OffsetPaginationRequest.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceListPendingInvitationsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceListPendingInvitationsRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.groupReference = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.pagination = OffsetPaginationRequest.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceListPendingInvitationsRequest {
    return {
      groupReference: isSet(object.groupReference) ? IdentityReference.fromJSON(object.groupReference) : undefined,
      pagination: isSet(object.pagination) ? OffsetPaginationRequest.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: GroupServiceListPendingInvitationsRequest): unknown {
    const obj: any = {};
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? IdentityReference.toJSON(message.groupReference) : undefined);
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationRequest.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceListPendingInvitationsRequest>, I>>(
    base?: I,
  ): GroupServiceListPendingInvitationsRequest {
    return GroupServiceListPendingInvitationsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceListPendingInvitationsRequest>, I>>(
    object: I,
  ): GroupServiceListPendingInvitationsRequest {
    const message = createBaseGroupServiceListPendingInvitationsRequest();
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? IdentityReference.fromPartial(object.groupReference)
      : undefined;
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationRequest.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseGroupServiceListPendingInvitationsResponse(): GroupServiceListPendingInvitationsResponse {
  return { invitations: [], pagination: undefined };
}

export const GroupServiceListPendingInvitationsResponse = {
  encode(message: GroupServiceListPendingInvitationsResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.invitations) {
      PendingGroupInvitation.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      OffsetPaginationResponse.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceListPendingInvitationsResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceListPendingInvitationsResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.invitations.push(PendingGroupInvitation.decode(reader, reader.uint32()));
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.pagination = OffsetPaginationResponse.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceListPendingInvitationsResponse {
    return {
      invitations: Array.isArray(object?.invitations)
        ? object.invitations.map((e: any) => PendingGroupInvitation.fromJSON(e))
        : [],
      pagination: isSet(object.pagination) ? OffsetPaginationResponse.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: GroupServiceListPendingInvitationsResponse): unknown {
    const obj: any = {};
    if (message.invitations) {
      obj.invitations = message.invitations.map((e) => e ? PendingGroupInvitation.toJSON(e) : undefined);
    } else {
      obj.invitations = [];
    }
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationResponse.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceListPendingInvitationsResponse>, I>>(
    base?: I,
  ): GroupServiceListPendingInvitationsResponse {
    return GroupServiceListPendingInvitationsResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceListPendingInvitationsResponse>, I>>(
    object: I,
  ): GroupServiceListPendingInvitationsResponse {
    const message = createBaseGroupServiceListPendingInvitationsResponse();
    message.invitations = object.invitations?.map((e) => PendingGroupInvitation.fromPartial(e)) || [];
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationResponse.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBasePendingGroupInvitation(): PendingGroupInvitation {
  return { userEmail: "", invitedBy: undefined, createdAt: undefined, invitationId: "" };
}

export const PendingGroupInvitation = {
  encode(message: PendingGroupInvitation, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.userEmail !== "") {
      writer.uint32(10).string(message.userEmail);
    }
    if (message.invitedBy !== undefined) {
      User.encode(message.invitedBy, writer.uint32(18).fork()).ldelim();
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(26).fork()).ldelim();
    }
    if (message.invitationId !== "") {
      writer.uint32(34).string(message.invitationId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): PendingGroupInvitation {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePendingGroupInvitation();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.userEmail = reader.string();
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.invitedBy = User.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.invitationId = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): PendingGroupInvitation {
    return {
      userEmail: isSet(object.userEmail) ? String(object.userEmail) : "",
      invitedBy: isSet(object.invitedBy) ? User.fromJSON(object.invitedBy) : undefined,
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      invitationId: isSet(object.invitationId) ? String(object.invitationId) : "",
    };
  },

  toJSON(message: PendingGroupInvitation): unknown {
    const obj: any = {};
    message.userEmail !== undefined && (obj.userEmail = message.userEmail);
    message.invitedBy !== undefined && (obj.invitedBy = message.invitedBy ? User.toJSON(message.invitedBy) : undefined);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.invitationId !== undefined && (obj.invitationId = message.invitationId);
    return obj;
  },

  create<I extends Exact<DeepPartial<PendingGroupInvitation>, I>>(base?: I): PendingGroupInvitation {
    return PendingGroupInvitation.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PendingGroupInvitation>, I>>(object: I): PendingGroupInvitation {
    const message = createBasePendingGroupInvitation();
    message.userEmail = object.userEmail ?? "";
    message.invitedBy = (object.invitedBy !== undefined && object.invitedBy !== null)
      ? User.fromPartial(object.invitedBy)
      : undefined;
    message.createdAt = object.createdAt ?? undefined;
    message.invitationId = object.invitationId ?? "";
    return message;
  },
};

function createBaseGroupMember(): GroupMember {
  return { user: undefined, isMaintainer: false, createdAt: undefined, updatedAt: undefined };
}

export const GroupMember = {
  encode(message: GroupMember, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.user !== undefined) {
      User.encode(message.user, writer.uint32(10).fork()).ldelim();
    }
    if (message.isMaintainer === true) {
      writer.uint32(16).bool(message.isMaintainer);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(26).fork()).ldelim();
    }
    if (message.updatedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.updatedAt), writer.uint32(34).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupMember {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupMember();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.user = User.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 16) {
            break;
          }

          message.isMaintainer = reader.bool();
          continue;
        case 3:
          if (tag !== 26) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.updatedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupMember {
    return {
      user: isSet(object.user) ? User.fromJSON(object.user) : undefined,
      isMaintainer: isSet(object.isMaintainer) ? Boolean(object.isMaintainer) : false,
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      updatedAt: isSet(object.updatedAt) ? fromJsonTimestamp(object.updatedAt) : undefined,
    };
  },

  toJSON(message: GroupMember): unknown {
    const obj: any = {};
    message.user !== undefined && (obj.user = message.user ? User.toJSON(message.user) : undefined);
    message.isMaintainer !== undefined && (obj.isMaintainer = message.isMaintainer);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.updatedAt !== undefined && (obj.updatedAt = message.updatedAt.toISOString());
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupMember>, I>>(base?: I): GroupMember {
    return GroupMember.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupMember>, I>>(object: I): GroupMember {
    const message = createBaseGroupMember();
    message.user = (object.user !== undefined && object.user !== null) ? User.fromPartial(object.user) : undefined;
    message.isMaintainer = object.isMaintainer ?? false;
    message.createdAt = object.createdAt ?? undefined;
    message.updatedAt = object.updatedAt ?? undefined;
    return message;
  },
};

function createBaseGroupServiceUpdateMemberMaintainerStatusRequest(): GroupServiceUpdateMemberMaintainerStatusRequest {
  return { groupReference: undefined, userId: "", isMaintainer: false };
}

export const GroupServiceUpdateMemberMaintainerStatusRequest = {
  encode(
    message: GroupServiceUpdateMemberMaintainerStatusRequest,
    writer: _m0.Writer = _m0.Writer.create(),
  ): _m0.Writer {
    if (message.groupReference !== undefined) {
      IdentityReference.encode(message.groupReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.userId !== "") {
      writer.uint32(18).string(message.userId);
    }
    if (message.isMaintainer === true) {
      writer.uint32(24).bool(message.isMaintainer);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceUpdateMemberMaintainerStatusRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceUpdateMemberMaintainerStatusRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.groupReference = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.userId = reader.string();
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.isMaintainer = reader.bool();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceUpdateMemberMaintainerStatusRequest {
    return {
      groupReference: isSet(object.groupReference) ? IdentityReference.fromJSON(object.groupReference) : undefined,
      userId: isSet(object.userId) ? String(object.userId) : "",
      isMaintainer: isSet(object.isMaintainer) ? Boolean(object.isMaintainer) : false,
    };
  },

  toJSON(message: GroupServiceUpdateMemberMaintainerStatusRequest): unknown {
    const obj: any = {};
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? IdentityReference.toJSON(message.groupReference) : undefined);
    message.userId !== undefined && (obj.userId = message.userId);
    message.isMaintainer !== undefined && (obj.isMaintainer = message.isMaintainer);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceUpdateMemberMaintainerStatusRequest>, I>>(
    base?: I,
  ): GroupServiceUpdateMemberMaintainerStatusRequest {
    return GroupServiceUpdateMemberMaintainerStatusRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceUpdateMemberMaintainerStatusRequest>, I>>(
    object: I,
  ): GroupServiceUpdateMemberMaintainerStatusRequest {
    const message = createBaseGroupServiceUpdateMemberMaintainerStatusRequest();
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? IdentityReference.fromPartial(object.groupReference)
      : undefined;
    message.userId = object.userId ?? "";
    message.isMaintainer = object.isMaintainer ?? false;
    return message;
  },
};

function createBaseGroupServiceUpdateMemberMaintainerStatusResponse(): GroupServiceUpdateMemberMaintainerStatusResponse {
  return {};
}

export const GroupServiceUpdateMemberMaintainerStatusResponse = {
  encode(_: GroupServiceUpdateMemberMaintainerStatusResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceUpdateMemberMaintainerStatusResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceUpdateMemberMaintainerStatusResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(_: any): GroupServiceUpdateMemberMaintainerStatusResponse {
    return {};
  },

  toJSON(_: GroupServiceUpdateMemberMaintainerStatusResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceUpdateMemberMaintainerStatusResponse>, I>>(
    base?: I,
  ): GroupServiceUpdateMemberMaintainerStatusResponse {
    return GroupServiceUpdateMemberMaintainerStatusResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceUpdateMemberMaintainerStatusResponse>, I>>(
    _: I,
  ): GroupServiceUpdateMemberMaintainerStatusResponse {
    const message = createBaseGroupServiceUpdateMemberMaintainerStatusResponse();
    return message;
  },
};

function createBaseGroupServiceListProjectsRequest(): GroupServiceListProjectsRequest {
  return { groupReference: undefined, pagination: undefined };
}

export const GroupServiceListProjectsRequest = {
  encode(message: GroupServiceListProjectsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.groupReference !== undefined) {
      IdentityReference.encode(message.groupReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      OffsetPaginationRequest.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceListProjectsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceListProjectsRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.groupReference = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.pagination = OffsetPaginationRequest.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceListProjectsRequest {
    return {
      groupReference: isSet(object.groupReference) ? IdentityReference.fromJSON(object.groupReference) : undefined,
      pagination: isSet(object.pagination) ? OffsetPaginationRequest.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: GroupServiceListProjectsRequest): unknown {
    const obj: any = {};
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? IdentityReference.toJSON(message.groupReference) : undefined);
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationRequest.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceListProjectsRequest>, I>>(base?: I): GroupServiceListProjectsRequest {
    return GroupServiceListProjectsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceListProjectsRequest>, I>>(
    object: I,
  ): GroupServiceListProjectsRequest {
    const message = createBaseGroupServiceListProjectsRequest();
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? IdentityReference.fromPartial(object.groupReference)
      : undefined;
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationRequest.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseGroupServiceListProjectsResponse(): GroupServiceListProjectsResponse {
  return { projects: [], pagination: undefined };
}

export const GroupServiceListProjectsResponse = {
  encode(message: GroupServiceListProjectsResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.projects) {
      ProjectInfo.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      OffsetPaginationResponse.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): GroupServiceListProjectsResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseGroupServiceListProjectsResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.projects.push(ProjectInfo.decode(reader, reader.uint32()));
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.pagination = OffsetPaginationResponse.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): GroupServiceListProjectsResponse {
    return {
      projects: Array.isArray(object?.projects) ? object.projects.map((e: any) => ProjectInfo.fromJSON(e)) : [],
      pagination: isSet(object.pagination) ? OffsetPaginationResponse.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: GroupServiceListProjectsResponse): unknown {
    const obj: any = {};
    if (message.projects) {
      obj.projects = message.projects.map((e) => e ? ProjectInfo.toJSON(e) : undefined);
    } else {
      obj.projects = [];
    }
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationResponse.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<GroupServiceListProjectsResponse>, I>>(
    base?: I,
  ): GroupServiceListProjectsResponse {
    return GroupServiceListProjectsResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<GroupServiceListProjectsResponse>, I>>(
    object: I,
  ): GroupServiceListProjectsResponse {
    const message = createBaseGroupServiceListProjectsResponse();
    message.projects = object.projects?.map((e) => ProjectInfo.fromPartial(e)) || [];
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationResponse.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseProjectInfo(): ProjectInfo {
  return { id: "", name: "", description: "", role: "", latestVersionId: undefined, createdAt: undefined };
}

export const ProjectInfo = {
  encode(message: ProjectInfo, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.id !== "") {
      writer.uint32(10).string(message.id);
    }
    if (message.name !== "") {
      writer.uint32(18).string(message.name);
    }
    if (message.description !== "") {
      writer.uint32(26).string(message.description);
    }
    if (message.role !== "") {
      writer.uint32(34).string(message.role);
    }
    if (message.latestVersionId !== undefined) {
      writer.uint32(42).string(message.latestVersionId);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(50).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectInfo {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectInfo();
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

          message.description = reader.string();
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.role = reader.string();
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.latestVersionId = reader.string();
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ProjectInfo {
    return {
      id: isSet(object.id) ? String(object.id) : "",
      name: isSet(object.name) ? String(object.name) : "",
      description: isSet(object.description) ? String(object.description) : "",
      role: isSet(object.role) ? String(object.role) : "",
      latestVersionId: isSet(object.latestVersionId) ? String(object.latestVersionId) : undefined,
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
    };
  },

  toJSON(message: ProjectInfo): unknown {
    const obj: any = {};
    message.id !== undefined && (obj.id = message.id);
    message.name !== undefined && (obj.name = message.name);
    message.description !== undefined && (obj.description = message.description);
    message.role !== undefined && (obj.role = message.role);
    message.latestVersionId !== undefined && (obj.latestVersionId = message.latestVersionId);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectInfo>, I>>(base?: I): ProjectInfo {
    return ProjectInfo.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectInfo>, I>>(object: I): ProjectInfo {
    const message = createBaseProjectInfo();
    message.id = object.id ?? "";
    message.name = object.name ?? "";
    message.description = object.description ?? "";
    message.role = object.role ?? "";
    message.latestVersionId = object.latestVersionId ?? undefined;
    message.createdAt = object.createdAt ?? undefined;
    return message;
  },
};

/** GroupService provides operations for managing groups within the system */
export interface GroupService {
  /** Create creates a new group with the specified name and description */
  Create(
    request: DeepPartial<GroupServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceCreateResponse>;
  /** Get retrieves a specific group by its ID */
  Get(request: DeepPartial<GroupServiceGetRequest>, metadata?: grpc.Metadata): Promise<GroupServiceGetResponse>;
  /** List retrieves a paginated list of groups, with optional filtering */
  List(request: DeepPartial<GroupServiceListRequest>, metadata?: grpc.Metadata): Promise<GroupServiceListResponse>;
  /** Update modifies an existing group's attributes */
  Update(
    request: DeepPartial<GroupServiceUpdateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceUpdateResponse>;
  /** Delete removes a group from the system */
  Delete(
    request: DeepPartial<GroupServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceDeleteResponse>;
  /** ListMembers retrieves the members of a specific group */
  ListMembers(
    request: DeepPartial<GroupServiceListMembersRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceListMembersResponse>;
  /** AddMember adds a user to a group with an optional maintainer role */
  AddMember(
    request: DeepPartial<GroupServiceAddMemberRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceAddMemberResponse>;
  /** RemoveMember removes a user from a group */
  RemoveMember(
    request: DeepPartial<GroupServiceRemoveMemberRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceRemoveMemberResponse>;
  /** UpdateMemberMaintainerStatus updates the maintainer status of a group member */
  UpdateMemberMaintainerStatus(
    request: DeepPartial<GroupServiceUpdateMemberMaintainerStatusRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceUpdateMemberMaintainerStatusResponse>;
  /** ListPendingInvitations retrieves pending invitations for a group */
  ListPendingInvitations(
    request: DeepPartial<GroupServiceListPendingInvitationsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceListPendingInvitationsResponse>;
  /** ListProjects retrieves a paginated list of projects the group is a member of */
  ListProjects(
    request: DeepPartial<GroupServiceListProjectsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceListProjectsResponse>;
}

export class GroupServiceClientImpl implements GroupService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.Create = this.Create.bind(this);
    this.Get = this.Get.bind(this);
    this.List = this.List.bind(this);
    this.Update = this.Update.bind(this);
    this.Delete = this.Delete.bind(this);
    this.ListMembers = this.ListMembers.bind(this);
    this.AddMember = this.AddMember.bind(this);
    this.RemoveMember = this.RemoveMember.bind(this);
    this.UpdateMemberMaintainerStatus = this.UpdateMemberMaintainerStatus.bind(this);
    this.ListPendingInvitations = this.ListPendingInvitations.bind(this);
    this.ListProjects = this.ListProjects.bind(this);
  }

  Create(
    request: DeepPartial<GroupServiceCreateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceCreateResponse> {
    return this.rpc.unary(GroupServiceCreateDesc, GroupServiceCreateRequest.fromPartial(request), metadata);
  }

  Get(request: DeepPartial<GroupServiceGetRequest>, metadata?: grpc.Metadata): Promise<GroupServiceGetResponse> {
    return this.rpc.unary(GroupServiceGetDesc, GroupServiceGetRequest.fromPartial(request), metadata);
  }

  List(request: DeepPartial<GroupServiceListRequest>, metadata?: grpc.Metadata): Promise<GroupServiceListResponse> {
    return this.rpc.unary(GroupServiceListDesc, GroupServiceListRequest.fromPartial(request), metadata);
  }

  Update(
    request: DeepPartial<GroupServiceUpdateRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceUpdateResponse> {
    return this.rpc.unary(GroupServiceUpdateDesc, GroupServiceUpdateRequest.fromPartial(request), metadata);
  }

  Delete(
    request: DeepPartial<GroupServiceDeleteRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceDeleteResponse> {
    return this.rpc.unary(GroupServiceDeleteDesc, GroupServiceDeleteRequest.fromPartial(request), metadata);
  }

  ListMembers(
    request: DeepPartial<GroupServiceListMembersRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceListMembersResponse> {
    return this.rpc.unary(GroupServiceListMembersDesc, GroupServiceListMembersRequest.fromPartial(request), metadata);
  }

  AddMember(
    request: DeepPartial<GroupServiceAddMemberRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceAddMemberResponse> {
    return this.rpc.unary(GroupServiceAddMemberDesc, GroupServiceAddMemberRequest.fromPartial(request), metadata);
  }

  RemoveMember(
    request: DeepPartial<GroupServiceRemoveMemberRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceRemoveMemberResponse> {
    return this.rpc.unary(GroupServiceRemoveMemberDesc, GroupServiceRemoveMemberRequest.fromPartial(request), metadata);
  }

  UpdateMemberMaintainerStatus(
    request: DeepPartial<GroupServiceUpdateMemberMaintainerStatusRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceUpdateMemberMaintainerStatusResponse> {
    return this.rpc.unary(
      GroupServiceUpdateMemberMaintainerStatusDesc,
      GroupServiceUpdateMemberMaintainerStatusRequest.fromPartial(request),
      metadata,
    );
  }

  ListPendingInvitations(
    request: DeepPartial<GroupServiceListPendingInvitationsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceListPendingInvitationsResponse> {
    return this.rpc.unary(
      GroupServiceListPendingInvitationsDesc,
      GroupServiceListPendingInvitationsRequest.fromPartial(request),
      metadata,
    );
  }

  ListProjects(
    request: DeepPartial<GroupServiceListProjectsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<GroupServiceListProjectsResponse> {
    return this.rpc.unary(GroupServiceListProjectsDesc, GroupServiceListProjectsRequest.fromPartial(request), metadata);
  }
}

export const GroupServiceDesc = { serviceName: "controlplane.v1.GroupService" };

export const GroupServiceCreateDesc: UnaryMethodDefinitionish = {
  methodName: "Create",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceCreateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceCreateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const GroupServiceGetDesc: UnaryMethodDefinitionish = {
  methodName: "Get",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceGetRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceGetResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const GroupServiceListDesc: UnaryMethodDefinitionish = {
  methodName: "List",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceListRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceListResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const GroupServiceUpdateDesc: UnaryMethodDefinitionish = {
  methodName: "Update",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceUpdateRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceUpdateResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const GroupServiceDeleteDesc: UnaryMethodDefinitionish = {
  methodName: "Delete",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceDeleteRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceDeleteResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const GroupServiceListMembersDesc: UnaryMethodDefinitionish = {
  methodName: "ListMembers",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceListMembersRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceListMembersResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const GroupServiceAddMemberDesc: UnaryMethodDefinitionish = {
  methodName: "AddMember",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceAddMemberRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceAddMemberResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const GroupServiceRemoveMemberDesc: UnaryMethodDefinitionish = {
  methodName: "RemoveMember",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceRemoveMemberRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceRemoveMemberResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const GroupServiceUpdateMemberMaintainerStatusDesc: UnaryMethodDefinitionish = {
  methodName: "UpdateMemberMaintainerStatus",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceUpdateMemberMaintainerStatusRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceUpdateMemberMaintainerStatusResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const GroupServiceListPendingInvitationsDesc: UnaryMethodDefinitionish = {
  methodName: "ListPendingInvitations",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceListPendingInvitationsRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceListPendingInvitationsResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const GroupServiceListProjectsDesc: UnaryMethodDefinitionish = {
  methodName: "ListProjects",
  service: GroupServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return GroupServiceListProjectsRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = GroupServiceListProjectsResponse.decode(data);
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

function isSet(value: any): boolean {
  return value !== null && value !== undefined;
}

export class GrpcWebError extends tsProtoGlobalThis.Error {
  constructor(message: string, public code: grpc.Code, public metadata: grpc.Metadata) {
    super(message);
  }
}
