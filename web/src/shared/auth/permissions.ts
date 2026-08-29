const modulePermissions = {
  articles: [
    'read', 'create', 'update', 'delete', 'update_all', 'delete_all', 'publish',
  ],
  comments: [
    'read', 'create', 'update', 'delete', 'update_all', 'delete_all', 'moderate',
  ],
  media: ['read', 'upload', 'update', 'delete'],
  categories: ['read', 'create', 'update', 'delete'],
  tags: ['read', 'create', 'update', 'delete'],
  menus: ['read', 'create', 'update', 'delete'],
  users: ['read', 'create', 'update', 'delete'],
  roles: ['read', 'create', 'update', 'delete'],
  settings: ['read', 'update'],
  seo: ['read', 'update'],
  analytics: ['read'],
  content: ['read', 'create', 'update', 'delete', 'publish'],
  content_types: ['read', 'create', 'update', 'delete'],
  plugins: ['read', 'update'],
  themes: ['read', 'update'],
  system: ['read', 'activity_log'],
  api_tokens: ['read', 'create', 'delete'],
  webhooks: ['read', 'create', 'delete'],
  backups: ['read', 'create', 'restore', 'delete'],
  tenants: ['read', 'manage'],
} as const

type ModuleName = keyof typeof modulePermissions
type ActionFor<M extends ModuleName> = (typeof modulePermissions)[M][number]

export type PermissionSlug = {
  [M in ModuleName]: `${M}.${ActionFor<M>}`
}[ModuleName]

function permission<M extends ModuleName, A extends ActionFor<M>>(
  module: M,
  action: A,
): `${M}.${A}` {
  return `${module}.${action}`
}

export const PERMISSIONS = {
  articles: {
    read: permission('articles', 'read'),
    create: permission('articles', 'create'),
    update: permission('articles', 'update'),
    delete: permission('articles', 'delete'),
    updateAll: permission('articles', 'update_all'),
    deleteAll: permission('articles', 'delete_all'),
    publish: permission('articles', 'publish'),
  },
  comments: {
    read: permission('comments', 'read'),
    create: permission('comments', 'create'),
    update: permission('comments', 'update'),
    delete: permission('comments', 'delete'),
    moderate: permission('comments', 'moderate'),
  },
  media: {
    read: permission('media', 'read'),
    upload: permission('media', 'upload'),
    update: permission('media', 'update'),
    delete: permission('media', 'delete'),
  },
  categories: {
    read: permission('categories', 'read'),
    create: permission('categories', 'create'),
    update: permission('categories', 'update'),
    delete: permission('categories', 'delete'),
  },
  tags: {
    read: permission('tags', 'read'),
    create: permission('tags', 'create'),
    update: permission('tags', 'update'),
    delete: permission('tags', 'delete'),
  },
  menus: {
    read: permission('menus', 'read'),
    create: permission('menus', 'create'),
    update: permission('menus', 'update'),
    delete: permission('menus', 'delete'),
  },
  users: {
    read: permission('users', 'read'),
    create: permission('users', 'create'),
    update: permission('users', 'update'),
    delete: permission('users', 'delete'),
  },
  roles: {
    read: permission('roles', 'read'),
    create: permission('roles', 'create'),
    update: permission('roles', 'update'),
    delete: permission('roles', 'delete'),
  },
  settings: {
    read: permission('settings', 'read'),
    update: permission('settings', 'update'),
  },
  seo: {
    read: permission('seo', 'read'),
    update: permission('seo', 'update'),
  },
  analytics: {
    read: permission('analytics', 'read'),
  },
  content: {
    read: permission('content', 'read'),
    create: permission('content', 'create'),
    update: permission('content', 'update'),
    delete: permission('content', 'delete'),
    publish: permission('content', 'publish'),
  },
  contentTypes: {
    read: permission('content_types', 'read'),
    create: permission('content_types', 'create'),
    update: permission('content_types', 'update'),
    delete: permission('content_types', 'delete'),
  },
  plugins: {
    read: permission('plugins', 'read'),
    update: permission('plugins', 'update'),
  },
  themes: {
    read: permission('themes', 'read'),
    update: permission('themes', 'update'),
  },
  system: {
    read: permission('system', 'read'),
    activityLog: permission('system', 'activity_log'),
  },
  apiTokens: {
    read: permission('api_tokens', 'read'),
    create: permission('api_tokens', 'create'),
    delete: permission('api_tokens', 'delete'),
  },
  webhooks: {
    read: permission('webhooks', 'read'),
    create: permission('webhooks', 'create'),
    delete: permission('webhooks', 'delete'),
  },
  backups: {
    read: permission('backups', 'read'),
    create: permission('backups', 'create'),
    restore: permission('backups', 'restore'),
    delete: permission('backups', 'delete'),
  },
  tenants: {
    read: permission('tenants', 'read'),
    manage: permission('tenants', 'manage'),
  },
} as const

export const ADMIN_WORKSPACE_PERMISSIONS: readonly PermissionSlug[] = [
  PERMISSIONS.articles.read,
  PERMISSIONS.articles.create,
  PERMISSIONS.comments.read,
  PERMISSIONS.media.read,
  PERMISSIONS.categories.read,
  PERMISSIONS.tags.read,
  PERMISSIONS.analytics.read,
  PERMISSIONS.content.read,
  PERMISSIONS.settings.read,
]

const permissionSet = new Set<string>(
  Object.entries(modulePermissions).flatMap(([module, actions]) =>
    actions.map((action) => `${module}.${action}`),
  ),
)

export function isPermissionSlug(value: string): value is PermissionSlug {
  return permissionSet.has(value)
}
