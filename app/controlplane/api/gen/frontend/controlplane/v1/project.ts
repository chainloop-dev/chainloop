/* eslint-disable */
import { grpc } from "@improbable-eng/grpc-web";
import { BrowserHeaders } from "browser-headers";
import _m0 from "protobufjs/minimal";
import { Timestamp } from "../../google/protobuf/timestamp";
import { Group } from "./group";
import { OffsetPaginationRequest, OffsetPaginationResponse } from "./pagination";
import { User } from "./response_messages";
import {
  IdentityReference,
  ProjectMemberRole,
  projectMemberRoleFromJSON,
  projectMemberRoleToJSON,
} from "./shared_message";

export const protobufPackage = "controlplane.v1";

/** ProjectServiceListMembersRequest contains the information needed to list members of a project */
export interface ProjectServiceListMembersRequest {
  /** IdentityReference is used to specify the project by either its ID or name */
  projectReference?: IdentityReference;
  /** Pagination parameters to limit and offset results */
  pagination?: OffsetPaginationRequest;
}

/** ProjectServiceListMembersResponse contains the list of members in a project */
export interface ProjectServiceListMembersResponse {
  /** The list of members in the project */
  members: ProjectMember[];
  /** Pagination information for the response */
  pagination?: OffsetPaginationResponse;
}

/** ProjectMember represents an user or group who is a member of a project */
export interface ProjectMember {
  /** The user who is a member of the project */
  user?:
    | User
    | undefined;
  /** The group who is a member of the project */
  group?:
    | Group
    | undefined;
  /** The role of the user in the project */
  role: ProjectMemberRole;
  /** Timestamp when the project membership was created */
  createdAt?: Date;
  /** Timestamp when the project membership was last modified */
  updatedAt?: Date;
  /** The ID of latest project version this member is associated with */
  latestProjectVersionId: string;
  /** Optional parent resource ID for nested project memberships */
  parentId?: string | undefined;
}

/** ProjectServiceAddMemberRequest contains the information needed to add a user to a project */
export interface ProjectServiceAddMemberRequest {
  /** IdentityReference is used to specify the project by either its ID or name */
  projectReference?: IdentityReference;
  /** The membership reference can be a user email or groups references in the future */
  memberReference?: ProjectMembershipReference;
  /** Indicates if the user should be added as an admin */
  role: ProjectMemberRole;
}

/** ProjectServiceAddMemberResponse contains the result of adding a user to a project */
export interface ProjectServiceAddMemberResponse {
}

export interface ProjectServiceRemoveMemberRequest {
  /** IdentityReference is used to specify the project by either its ID or name */
  projectReference?: IdentityReference;
  /** The membership reference can be a user email or groups references in the future */
  memberReference?: ProjectMembershipReference;
}

/** ProjectServiceRemoveMemberResponse is returned upon successful removal of a user from a project */
export interface ProjectServiceRemoveMemberResponse {
}

/** ProjectMembershipReference is used to reference a user or group in the context of project membership */
export interface ProjectMembershipReference {
  /** The user to add to the project */
  userEmail?:
    | string
    | undefined;
  /** The group to add to the project */
  groupReference?: IdentityReference | undefined;
}

/** ProjectServiceUpdateMemberRoleRequest contains the information needed to update a member's role in a project */
export interface ProjectServiceUpdateMemberRoleRequest {
  /** IdentityReference is used to specify the project by either its ID or name */
  projectReference?: IdentityReference;
  /** The membership reference can be a user email or groups references in the future */
  memberReference?: ProjectMembershipReference;
  /** The new role for the member in the project */
  newRole: ProjectMemberRole;
}

/** ProjectServiceUpdateMemberRoleResponse is returned upon successful update of a member's role in a project */
export interface ProjectServiceUpdateMemberRoleResponse {
}

export interface ProjectServiceListPendingInvitationsRequest {
  /** IdentityReference is used to specify the project by either its ID or name */
  projectReference?: IdentityReference;
  /** Pagination parameters to limit and offset results */
  pagination?: OffsetPaginationRequest;
}

/** ProjectServiceListPendingInvitationsResponse contains a list of pending invitations for a project */
export interface ProjectServiceListPendingInvitationsResponse {
  /** List of pending invitations for the project */
  invitations: PendingProjectInvitation[];
  /** Pagination information for the response */
  pagination?: OffsetPaginationResponse;
}

/** PendingInvitation represents an invitation to join a project that has not yet been accepted */
export interface PendingProjectInvitation {
  /** The email address of the user invited to the project */
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

function createBaseProjectServiceListMembersRequest(): ProjectServiceListMembersRequest {
  return { projectReference: undefined, pagination: undefined };
}

export const ProjectServiceListMembersRequest = {
  encode(message: ProjectServiceListMembersRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.projectReference !== undefined) {
      IdentityReference.encode(message.projectReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      OffsetPaginationRequest.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceListMembersRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceListMembersRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.projectReference = IdentityReference.decode(reader, reader.uint32());
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

  fromJSON(object: any): ProjectServiceListMembersRequest {
    return {
      projectReference: isSet(object.projectReference)
        ? IdentityReference.fromJSON(object.projectReference)
        : undefined,
      pagination: isSet(object.pagination) ? OffsetPaginationRequest.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: ProjectServiceListMembersRequest): unknown {
    const obj: any = {};
    message.projectReference !== undefined &&
      (obj.projectReference = message.projectReference
        ? IdentityReference.toJSON(message.projectReference)
        : undefined);
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationRequest.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceListMembersRequest>, I>>(
    base?: I,
  ): ProjectServiceListMembersRequest {
    return ProjectServiceListMembersRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceListMembersRequest>, I>>(
    object: I,
  ): ProjectServiceListMembersRequest {
    const message = createBaseProjectServiceListMembersRequest();
    message.projectReference = (object.projectReference !== undefined && object.projectReference !== null)
      ? IdentityReference.fromPartial(object.projectReference)
      : undefined;
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationRequest.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseProjectServiceListMembersResponse(): ProjectServiceListMembersResponse {
  return { members: [], pagination: undefined };
}

export const ProjectServiceListMembersResponse = {
  encode(message: ProjectServiceListMembersResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.members) {
      ProjectMember.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      OffsetPaginationResponse.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceListMembersResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceListMembersResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.members.push(ProjectMember.decode(reader, reader.uint32()));
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

  fromJSON(object: any): ProjectServiceListMembersResponse {
    return {
      members: Array.isArray(object?.members) ? object.members.map((e: any) => ProjectMember.fromJSON(e)) : [],
      pagination: isSet(object.pagination) ? OffsetPaginationResponse.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: ProjectServiceListMembersResponse): unknown {
    const obj: any = {};
    if (message.members) {
      obj.members = message.members.map((e) => e ? ProjectMember.toJSON(e) : undefined);
    } else {
      obj.members = [];
    }
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationResponse.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceListMembersResponse>, I>>(
    base?: I,
  ): ProjectServiceListMembersResponse {
    return ProjectServiceListMembersResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceListMembersResponse>, I>>(
    object: I,
  ): ProjectServiceListMembersResponse {
    const message = createBaseProjectServiceListMembersResponse();
    message.members = object.members?.map((e) => ProjectMember.fromPartial(e)) || [];
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationResponse.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseProjectMember(): ProjectMember {
  return {
    user: undefined,
    group: undefined,
    role: 0,
    createdAt: undefined,
    updatedAt: undefined,
    latestProjectVersionId: "",
    parentId: undefined,
  };
}

export const ProjectMember = {
  encode(message: ProjectMember, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.user !== undefined) {
      User.encode(message.user, writer.uint32(10).fork()).ldelim();
    }
    if (message.group !== undefined) {
      Group.encode(message.group, writer.uint32(18).fork()).ldelim();
    }
    if (message.role !== 0) {
      writer.uint32(24).int32(message.role);
    }
    if (message.createdAt !== undefined) {
      Timestamp.encode(toTimestamp(message.createdAt), writer.uint32(34).fork()).ldelim();
    }
    if (message.updatedAt !== undefined) {
      Timestamp.encode(toTimestamp(message.updatedAt), writer.uint32(42).fork()).ldelim();
    }
    if (message.latestProjectVersionId !== "") {
      writer.uint32(50).string(message.latestProjectVersionId);
    }
    if (message.parentId !== undefined) {
      writer.uint32(58).string(message.parentId);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectMember {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectMember();
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
          if (tag !== 18) {
            break;
          }

          message.group = Group.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.role = reader.int32() as any;
          continue;
        case 4:
          if (tag !== 34) {
            break;
          }

          message.createdAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 5:
          if (tag !== 42) {
            break;
          }

          message.updatedAt = fromTimestamp(Timestamp.decode(reader, reader.uint32()));
          continue;
        case 6:
          if (tag !== 50) {
            break;
          }

          message.latestProjectVersionId = reader.string();
          continue;
        case 7:
          if (tag !== 58) {
            break;
          }

          message.parentId = reader.string();
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ProjectMember {
    return {
      user: isSet(object.user) ? User.fromJSON(object.user) : undefined,
      group: isSet(object.group) ? Group.fromJSON(object.group) : undefined,
      role: isSet(object.role) ? projectMemberRoleFromJSON(object.role) : 0,
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      updatedAt: isSet(object.updatedAt) ? fromJsonTimestamp(object.updatedAt) : undefined,
      latestProjectVersionId: isSet(object.latestProjectVersionId) ? String(object.latestProjectVersionId) : "",
      parentId: isSet(object.parentId) ? String(object.parentId) : undefined,
    };
  },

  toJSON(message: ProjectMember): unknown {
    const obj: any = {};
    message.user !== undefined && (obj.user = message.user ? User.toJSON(message.user) : undefined);
    message.group !== undefined && (obj.group = message.group ? Group.toJSON(message.group) : undefined);
    message.role !== undefined && (obj.role = projectMemberRoleToJSON(message.role));
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.updatedAt !== undefined && (obj.updatedAt = message.updatedAt.toISOString());
    message.latestProjectVersionId !== undefined && (obj.latestProjectVersionId = message.latestProjectVersionId);
    message.parentId !== undefined && (obj.parentId = message.parentId);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectMember>, I>>(base?: I): ProjectMember {
    return ProjectMember.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectMember>, I>>(object: I): ProjectMember {
    const message = createBaseProjectMember();
    message.user = (object.user !== undefined && object.user !== null) ? User.fromPartial(object.user) : undefined;
    message.group = (object.group !== undefined && object.group !== null) ? Group.fromPartial(object.group) : undefined;
    message.role = object.role ?? 0;
    message.createdAt = object.createdAt ?? undefined;
    message.updatedAt = object.updatedAt ?? undefined;
    message.latestProjectVersionId = object.latestProjectVersionId ?? "";
    message.parentId = object.parentId ?? undefined;
    return message;
  },
};

function createBaseProjectServiceAddMemberRequest(): ProjectServiceAddMemberRequest {
  return { projectReference: undefined, memberReference: undefined, role: 0 };
}

export const ProjectServiceAddMemberRequest = {
  encode(message: ProjectServiceAddMemberRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.projectReference !== undefined) {
      IdentityReference.encode(message.projectReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.memberReference !== undefined) {
      ProjectMembershipReference.encode(message.memberReference, writer.uint32(18).fork()).ldelim();
    }
    if (message.role !== 0) {
      writer.uint32(24).int32(message.role);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceAddMemberRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceAddMemberRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.projectReference = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.memberReference = ProjectMembershipReference.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.role = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ProjectServiceAddMemberRequest {
    return {
      projectReference: isSet(object.projectReference)
        ? IdentityReference.fromJSON(object.projectReference)
        : undefined,
      memberReference: isSet(object.memberReference)
        ? ProjectMembershipReference.fromJSON(object.memberReference)
        : undefined,
      role: isSet(object.role) ? projectMemberRoleFromJSON(object.role) : 0,
    };
  },

  toJSON(message: ProjectServiceAddMemberRequest): unknown {
    const obj: any = {};
    message.projectReference !== undefined &&
      (obj.projectReference = message.projectReference
        ? IdentityReference.toJSON(message.projectReference)
        : undefined);
    message.memberReference !== undefined && (obj.memberReference = message.memberReference
      ? ProjectMembershipReference.toJSON(message.memberReference)
      : undefined);
    message.role !== undefined && (obj.role = projectMemberRoleToJSON(message.role));
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceAddMemberRequest>, I>>(base?: I): ProjectServiceAddMemberRequest {
    return ProjectServiceAddMemberRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceAddMemberRequest>, I>>(
    object: I,
  ): ProjectServiceAddMemberRequest {
    const message = createBaseProjectServiceAddMemberRequest();
    message.projectReference = (object.projectReference !== undefined && object.projectReference !== null)
      ? IdentityReference.fromPartial(object.projectReference)
      : undefined;
    message.memberReference = (object.memberReference !== undefined && object.memberReference !== null)
      ? ProjectMembershipReference.fromPartial(object.memberReference)
      : undefined;
    message.role = object.role ?? 0;
    return message;
  },
};

function createBaseProjectServiceAddMemberResponse(): ProjectServiceAddMemberResponse {
  return {};
}

export const ProjectServiceAddMemberResponse = {
  encode(_: ProjectServiceAddMemberResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceAddMemberResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceAddMemberResponse();
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

  fromJSON(_: any): ProjectServiceAddMemberResponse {
    return {};
  },

  toJSON(_: ProjectServiceAddMemberResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceAddMemberResponse>, I>>(base?: I): ProjectServiceAddMemberResponse {
    return ProjectServiceAddMemberResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceAddMemberResponse>, I>>(_: I): ProjectServiceAddMemberResponse {
    const message = createBaseProjectServiceAddMemberResponse();
    return message;
  },
};

function createBaseProjectServiceRemoveMemberRequest(): ProjectServiceRemoveMemberRequest {
  return { projectReference: undefined, memberReference: undefined };
}

export const ProjectServiceRemoveMemberRequest = {
  encode(message: ProjectServiceRemoveMemberRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.projectReference !== undefined) {
      IdentityReference.encode(message.projectReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.memberReference !== undefined) {
      ProjectMembershipReference.encode(message.memberReference, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceRemoveMemberRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceRemoveMemberRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.projectReference = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.memberReference = ProjectMembershipReference.decode(reader, reader.uint32());
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ProjectServiceRemoveMemberRequest {
    return {
      projectReference: isSet(object.projectReference)
        ? IdentityReference.fromJSON(object.projectReference)
        : undefined,
      memberReference: isSet(object.memberReference)
        ? ProjectMembershipReference.fromJSON(object.memberReference)
        : undefined,
    };
  },

  toJSON(message: ProjectServiceRemoveMemberRequest): unknown {
    const obj: any = {};
    message.projectReference !== undefined &&
      (obj.projectReference = message.projectReference
        ? IdentityReference.toJSON(message.projectReference)
        : undefined);
    message.memberReference !== undefined && (obj.memberReference = message.memberReference
      ? ProjectMembershipReference.toJSON(message.memberReference)
      : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceRemoveMemberRequest>, I>>(
    base?: I,
  ): ProjectServiceRemoveMemberRequest {
    return ProjectServiceRemoveMemberRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceRemoveMemberRequest>, I>>(
    object: I,
  ): ProjectServiceRemoveMemberRequest {
    const message = createBaseProjectServiceRemoveMemberRequest();
    message.projectReference = (object.projectReference !== undefined && object.projectReference !== null)
      ? IdentityReference.fromPartial(object.projectReference)
      : undefined;
    message.memberReference = (object.memberReference !== undefined && object.memberReference !== null)
      ? ProjectMembershipReference.fromPartial(object.memberReference)
      : undefined;
    return message;
  },
};

function createBaseProjectServiceRemoveMemberResponse(): ProjectServiceRemoveMemberResponse {
  return {};
}

export const ProjectServiceRemoveMemberResponse = {
  encode(_: ProjectServiceRemoveMemberResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceRemoveMemberResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceRemoveMemberResponse();
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

  fromJSON(_: any): ProjectServiceRemoveMemberResponse {
    return {};
  },

  toJSON(_: ProjectServiceRemoveMemberResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceRemoveMemberResponse>, I>>(
    base?: I,
  ): ProjectServiceRemoveMemberResponse {
    return ProjectServiceRemoveMemberResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceRemoveMemberResponse>, I>>(
    _: I,
  ): ProjectServiceRemoveMemberResponse {
    const message = createBaseProjectServiceRemoveMemberResponse();
    return message;
  },
};

function createBaseProjectMembershipReference(): ProjectMembershipReference {
  return { userEmail: undefined, groupReference: undefined };
}

export const ProjectMembershipReference = {
  encode(message: ProjectMembershipReference, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.userEmail !== undefined) {
      writer.uint32(10).string(message.userEmail);
    }
    if (message.groupReference !== undefined) {
      IdentityReference.encode(message.groupReference, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectMembershipReference {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectMembershipReference();
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

  fromJSON(object: any): ProjectMembershipReference {
    return {
      userEmail: isSet(object.userEmail) ? String(object.userEmail) : undefined,
      groupReference: isSet(object.groupReference) ? IdentityReference.fromJSON(object.groupReference) : undefined,
    };
  },

  toJSON(message: ProjectMembershipReference): unknown {
    const obj: any = {};
    message.userEmail !== undefined && (obj.userEmail = message.userEmail);
    message.groupReference !== undefined &&
      (obj.groupReference = message.groupReference ? IdentityReference.toJSON(message.groupReference) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectMembershipReference>, I>>(base?: I): ProjectMembershipReference {
    return ProjectMembershipReference.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectMembershipReference>, I>>(object: I): ProjectMembershipReference {
    const message = createBaseProjectMembershipReference();
    message.userEmail = object.userEmail ?? undefined;
    message.groupReference = (object.groupReference !== undefined && object.groupReference !== null)
      ? IdentityReference.fromPartial(object.groupReference)
      : undefined;
    return message;
  },
};

function createBaseProjectServiceUpdateMemberRoleRequest(): ProjectServiceUpdateMemberRoleRequest {
  return { projectReference: undefined, memberReference: undefined, newRole: 0 };
}

export const ProjectServiceUpdateMemberRoleRequest = {
  encode(message: ProjectServiceUpdateMemberRoleRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.projectReference !== undefined) {
      IdentityReference.encode(message.projectReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.memberReference !== undefined) {
      ProjectMembershipReference.encode(message.memberReference, writer.uint32(18).fork()).ldelim();
    }
    if (message.newRole !== 0) {
      writer.uint32(24).int32(message.newRole);
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceUpdateMemberRoleRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceUpdateMemberRoleRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.projectReference = IdentityReference.decode(reader, reader.uint32());
          continue;
        case 2:
          if (tag !== 18) {
            break;
          }

          message.memberReference = ProjectMembershipReference.decode(reader, reader.uint32());
          continue;
        case 3:
          if (tag !== 24) {
            break;
          }

          message.newRole = reader.int32() as any;
          continue;
      }
      if ((tag & 7) === 4 || tag === 0) {
        break;
      }
      reader.skipType(tag & 7);
    }
    return message;
  },

  fromJSON(object: any): ProjectServiceUpdateMemberRoleRequest {
    return {
      projectReference: isSet(object.projectReference)
        ? IdentityReference.fromJSON(object.projectReference)
        : undefined,
      memberReference: isSet(object.memberReference)
        ? ProjectMembershipReference.fromJSON(object.memberReference)
        : undefined,
      newRole: isSet(object.newRole) ? projectMemberRoleFromJSON(object.newRole) : 0,
    };
  },

  toJSON(message: ProjectServiceUpdateMemberRoleRequest): unknown {
    const obj: any = {};
    message.projectReference !== undefined &&
      (obj.projectReference = message.projectReference
        ? IdentityReference.toJSON(message.projectReference)
        : undefined);
    message.memberReference !== undefined && (obj.memberReference = message.memberReference
      ? ProjectMembershipReference.toJSON(message.memberReference)
      : undefined);
    message.newRole !== undefined && (obj.newRole = projectMemberRoleToJSON(message.newRole));
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceUpdateMemberRoleRequest>, I>>(
    base?: I,
  ): ProjectServiceUpdateMemberRoleRequest {
    return ProjectServiceUpdateMemberRoleRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceUpdateMemberRoleRequest>, I>>(
    object: I,
  ): ProjectServiceUpdateMemberRoleRequest {
    const message = createBaseProjectServiceUpdateMemberRoleRequest();
    message.projectReference = (object.projectReference !== undefined && object.projectReference !== null)
      ? IdentityReference.fromPartial(object.projectReference)
      : undefined;
    message.memberReference = (object.memberReference !== undefined && object.memberReference !== null)
      ? ProjectMembershipReference.fromPartial(object.memberReference)
      : undefined;
    message.newRole = object.newRole ?? 0;
    return message;
  },
};

function createBaseProjectServiceUpdateMemberRoleResponse(): ProjectServiceUpdateMemberRoleResponse {
  return {};
}

export const ProjectServiceUpdateMemberRoleResponse = {
  encode(_: ProjectServiceUpdateMemberRoleResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceUpdateMemberRoleResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceUpdateMemberRoleResponse();
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

  fromJSON(_: any): ProjectServiceUpdateMemberRoleResponse {
    return {};
  },

  toJSON(_: ProjectServiceUpdateMemberRoleResponse): unknown {
    const obj: any = {};
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceUpdateMemberRoleResponse>, I>>(
    base?: I,
  ): ProjectServiceUpdateMemberRoleResponse {
    return ProjectServiceUpdateMemberRoleResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceUpdateMemberRoleResponse>, I>>(
    _: I,
  ): ProjectServiceUpdateMemberRoleResponse {
    const message = createBaseProjectServiceUpdateMemberRoleResponse();
    return message;
  },
};

function createBaseProjectServiceListPendingInvitationsRequest(): ProjectServiceListPendingInvitationsRequest {
  return { projectReference: undefined, pagination: undefined };
}

export const ProjectServiceListPendingInvitationsRequest = {
  encode(message: ProjectServiceListPendingInvitationsRequest, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    if (message.projectReference !== undefined) {
      IdentityReference.encode(message.projectReference, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      OffsetPaginationRequest.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceListPendingInvitationsRequest {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceListPendingInvitationsRequest();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.projectReference = IdentityReference.decode(reader, reader.uint32());
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

  fromJSON(object: any): ProjectServiceListPendingInvitationsRequest {
    return {
      projectReference: isSet(object.projectReference)
        ? IdentityReference.fromJSON(object.projectReference)
        : undefined,
      pagination: isSet(object.pagination) ? OffsetPaginationRequest.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: ProjectServiceListPendingInvitationsRequest): unknown {
    const obj: any = {};
    message.projectReference !== undefined &&
      (obj.projectReference = message.projectReference
        ? IdentityReference.toJSON(message.projectReference)
        : undefined);
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationRequest.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceListPendingInvitationsRequest>, I>>(
    base?: I,
  ): ProjectServiceListPendingInvitationsRequest {
    return ProjectServiceListPendingInvitationsRequest.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceListPendingInvitationsRequest>, I>>(
    object: I,
  ): ProjectServiceListPendingInvitationsRequest {
    const message = createBaseProjectServiceListPendingInvitationsRequest();
    message.projectReference = (object.projectReference !== undefined && object.projectReference !== null)
      ? IdentityReference.fromPartial(object.projectReference)
      : undefined;
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationRequest.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBaseProjectServiceListPendingInvitationsResponse(): ProjectServiceListPendingInvitationsResponse {
  return { invitations: [], pagination: undefined };
}

export const ProjectServiceListPendingInvitationsResponse = {
  encode(message: ProjectServiceListPendingInvitationsResponse, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
    for (const v of message.invitations) {
      PendingProjectInvitation.encode(v!, writer.uint32(10).fork()).ldelim();
    }
    if (message.pagination !== undefined) {
      OffsetPaginationResponse.encode(message.pagination, writer.uint32(18).fork()).ldelim();
    }
    return writer;
  },

  decode(input: _m0.Reader | Uint8Array, length?: number): ProjectServiceListPendingInvitationsResponse {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBaseProjectServiceListPendingInvitationsResponse();
    while (reader.pos < end) {
      const tag = reader.uint32();
      switch (tag >>> 3) {
        case 1:
          if (tag !== 10) {
            break;
          }

          message.invitations.push(PendingProjectInvitation.decode(reader, reader.uint32()));
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

  fromJSON(object: any): ProjectServiceListPendingInvitationsResponse {
    return {
      invitations: Array.isArray(object?.invitations)
        ? object.invitations.map((e: any) => PendingProjectInvitation.fromJSON(e))
        : [],
      pagination: isSet(object.pagination) ? OffsetPaginationResponse.fromJSON(object.pagination) : undefined,
    };
  },

  toJSON(message: ProjectServiceListPendingInvitationsResponse): unknown {
    const obj: any = {};
    if (message.invitations) {
      obj.invitations = message.invitations.map((e) => e ? PendingProjectInvitation.toJSON(e) : undefined);
    } else {
      obj.invitations = [];
    }
    message.pagination !== undefined &&
      (obj.pagination = message.pagination ? OffsetPaginationResponse.toJSON(message.pagination) : undefined);
    return obj;
  },

  create<I extends Exact<DeepPartial<ProjectServiceListPendingInvitationsResponse>, I>>(
    base?: I,
  ): ProjectServiceListPendingInvitationsResponse {
    return ProjectServiceListPendingInvitationsResponse.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<ProjectServiceListPendingInvitationsResponse>, I>>(
    object: I,
  ): ProjectServiceListPendingInvitationsResponse {
    const message = createBaseProjectServiceListPendingInvitationsResponse();
    message.invitations = object.invitations?.map((e) => PendingProjectInvitation.fromPartial(e)) || [];
    message.pagination = (object.pagination !== undefined && object.pagination !== null)
      ? OffsetPaginationResponse.fromPartial(object.pagination)
      : undefined;
    return message;
  },
};

function createBasePendingProjectInvitation(): PendingProjectInvitation {
  return { userEmail: "", invitedBy: undefined, createdAt: undefined, invitationId: "" };
}

export const PendingProjectInvitation = {
  encode(message: PendingProjectInvitation, writer: _m0.Writer = _m0.Writer.create()): _m0.Writer {
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

  decode(input: _m0.Reader | Uint8Array, length?: number): PendingProjectInvitation {
    const reader = input instanceof _m0.Reader ? input : _m0.Reader.create(input);
    let end = length === undefined ? reader.len : reader.pos + length;
    const message = createBasePendingProjectInvitation();
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

  fromJSON(object: any): PendingProjectInvitation {
    return {
      userEmail: isSet(object.userEmail) ? String(object.userEmail) : "",
      invitedBy: isSet(object.invitedBy) ? User.fromJSON(object.invitedBy) : undefined,
      createdAt: isSet(object.createdAt) ? fromJsonTimestamp(object.createdAt) : undefined,
      invitationId: isSet(object.invitationId) ? String(object.invitationId) : "",
    };
  },

  toJSON(message: PendingProjectInvitation): unknown {
    const obj: any = {};
    message.userEmail !== undefined && (obj.userEmail = message.userEmail);
    message.invitedBy !== undefined && (obj.invitedBy = message.invitedBy ? User.toJSON(message.invitedBy) : undefined);
    message.createdAt !== undefined && (obj.createdAt = message.createdAt.toISOString());
    message.invitationId !== undefined && (obj.invitationId = message.invitationId);
    return obj;
  },

  create<I extends Exact<DeepPartial<PendingProjectInvitation>, I>>(base?: I): PendingProjectInvitation {
    return PendingProjectInvitation.fromPartial(base ?? {});
  },

  fromPartial<I extends Exact<DeepPartial<PendingProjectInvitation>, I>>(object: I): PendingProjectInvitation {
    const message = createBasePendingProjectInvitation();
    message.userEmail = object.userEmail ?? "";
    message.invitedBy = (object.invitedBy !== undefined && object.invitedBy !== null)
      ? User.fromPartial(object.invitedBy)
      : undefined;
    message.createdAt = object.createdAt ?? undefined;
    message.invitationId = object.invitationId ?? "";
    return message;
  },
};

export interface ProjectService {
  /** Project membership management */
  ListMembers(
    request: DeepPartial<ProjectServiceListMembersRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceListMembersResponse>;
  AddMember(
    request: DeepPartial<ProjectServiceAddMemberRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceAddMemberResponse>;
  RemoveMember(
    request: DeepPartial<ProjectServiceRemoveMemberRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceRemoveMemberResponse>;
  UpdateMemberRole(
    request: DeepPartial<ProjectServiceUpdateMemberRoleRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceUpdateMemberRoleResponse>;
  ListPendingInvitations(
    request: DeepPartial<ProjectServiceListPendingInvitationsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceListPendingInvitationsResponse>;
}

export class ProjectServiceClientImpl implements ProjectService {
  private readonly rpc: Rpc;

  constructor(rpc: Rpc) {
    this.rpc = rpc;
    this.ListMembers = this.ListMembers.bind(this);
    this.AddMember = this.AddMember.bind(this);
    this.RemoveMember = this.RemoveMember.bind(this);
    this.UpdateMemberRole = this.UpdateMemberRole.bind(this);
    this.ListPendingInvitations = this.ListPendingInvitations.bind(this);
  }

  ListMembers(
    request: DeepPartial<ProjectServiceListMembersRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceListMembersResponse> {
    return this.rpc.unary(
      ProjectServiceListMembersDesc,
      ProjectServiceListMembersRequest.fromPartial(request),
      metadata,
    );
  }

  AddMember(
    request: DeepPartial<ProjectServiceAddMemberRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceAddMemberResponse> {
    return this.rpc.unary(ProjectServiceAddMemberDesc, ProjectServiceAddMemberRequest.fromPartial(request), metadata);
  }

  RemoveMember(
    request: DeepPartial<ProjectServiceRemoveMemberRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceRemoveMemberResponse> {
    return this.rpc.unary(
      ProjectServiceRemoveMemberDesc,
      ProjectServiceRemoveMemberRequest.fromPartial(request),
      metadata,
    );
  }

  UpdateMemberRole(
    request: DeepPartial<ProjectServiceUpdateMemberRoleRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceUpdateMemberRoleResponse> {
    return this.rpc.unary(
      ProjectServiceUpdateMemberRoleDesc,
      ProjectServiceUpdateMemberRoleRequest.fromPartial(request),
      metadata,
    );
  }

  ListPendingInvitations(
    request: DeepPartial<ProjectServiceListPendingInvitationsRequest>,
    metadata?: grpc.Metadata,
  ): Promise<ProjectServiceListPendingInvitationsResponse> {
    return this.rpc.unary(
      ProjectServiceListPendingInvitationsDesc,
      ProjectServiceListPendingInvitationsRequest.fromPartial(request),
      metadata,
    );
  }
}

export const ProjectServiceDesc = { serviceName: "controlplane.v1.ProjectService" };

export const ProjectServiceListMembersDesc: UnaryMethodDefinitionish = {
  methodName: "ListMembers",
  service: ProjectServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ProjectServiceListMembersRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ProjectServiceListMembersResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const ProjectServiceAddMemberDesc: UnaryMethodDefinitionish = {
  methodName: "AddMember",
  service: ProjectServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ProjectServiceAddMemberRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ProjectServiceAddMemberResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const ProjectServiceRemoveMemberDesc: UnaryMethodDefinitionish = {
  methodName: "RemoveMember",
  service: ProjectServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ProjectServiceRemoveMemberRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ProjectServiceRemoveMemberResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const ProjectServiceUpdateMemberRoleDesc: UnaryMethodDefinitionish = {
  methodName: "UpdateMemberRole",
  service: ProjectServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ProjectServiceUpdateMemberRoleRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ProjectServiceUpdateMemberRoleResponse.decode(data);
      return {
        ...value,
        toObject() {
          return value;
        },
      };
    },
  } as any,
};

export const ProjectServiceListPendingInvitationsDesc: UnaryMethodDefinitionish = {
  methodName: "ListPendingInvitations",
  service: ProjectServiceDesc,
  requestStream: false,
  responseStream: false,
  requestType: {
    serializeBinary() {
      return ProjectServiceListPendingInvitationsRequest.encode(this).finish();
    },
  } as any,
  responseType: {
    deserializeBinary(data: Uint8Array) {
      const value = ProjectServiceListPendingInvitationsResponse.decode(data);
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
