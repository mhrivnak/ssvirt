# VM Management API Guide for UI Clients

## Overview

This guide provides complete details for UI client developers to implement virtual machine creation and management functionality using the SSVirt CloudAPI endpoints. The APIs follow VMware Cloud Director patterns while being powered by OpenShift Virtualization.

## Table of Contents

1. [Authentication & Authorization](#authentication--authorization)
2. [Core Concepts](#core-concepts)
3. [API Endpoints Overview](#api-endpoints-overview)
4. [VM Creation Workflow](#vm-creation-workflow)
5. [vApp Management](#vapp-management)
6. [VM Details](#vm-details)
7. [Error Handling](#error-handling)
8. [UI Implementation Patterns](#ui-implementation-patterns)
9. [Code Examples](#code-examples)

## Authentication & Authorization

### JWT Token Authentication

All API endpoints require JWT authentication via the `Authorization` header:

```http
Authorization: Bearer <jwt-token>
```

### Organization-Based Access Control

Users can only access resources (VDCs, vApps, VMs) that belong to their organization. The system automatically filters results based on the authenticated user's organization membership.

**Access Hierarchy:**
- **User** → belongs to → **Organization**
- **Organization** → contains → **VDCs**
- **VDCs** → contain → **vApps**
- **vApps** → contain → **VMs**

## Core Concepts

### Resource Types

| Resource | Description | Purpose |
|----------|-------------|---------|
| **VDC** | Virtual Data Center | Container for vApps, maps to Kubernetes namespace |
| **vApp** | Virtual Application | Container for VMs, maps to OpenShift TemplateInstance |
| **VM** | Virtual Machine | Individual virtual machine, maps to OpenShift VirtualMachine |
| **Catalog Item** | VM Template | Template for creating VMs, maps to OpenShift Template |

### Resource Relationships

```
Organization
├── VDC (Virtual Data Center)
│   ├── vApp (Virtual Application)
│   │   ├── VM (Virtual Machine)
│   │   └── VM (Virtual Machine)
│   └── vApp
│       └── VM
└── Catalog
    ├── Catalog Item (Template)
    └── Catalog Item (Template)
```

### VM Lifecycle States

| Status | Description | User Actions Available |
|--------|-------------|------------------------|
| `INSTANTIATING` | VM is being created | Wait, view details |
| `RESOLVED` | Template resolved, VM defined | Power on |
| `DEPLOYED` | VM deployed but powered off | Power on, delete |
| `POWERED_ON` | VM is running | Power off, view details, delete |
| `POWERED_OFF` | VM is stopped | Power on, delete |
| `POWERING_ON` | VM is starting | Wait |
| `POWERING_OFF` | VM is stopping | Wait |
| `SUSPENDED` | VM is suspended | Resume, delete |
| `FAILED` | VM creation/operation failed | Delete, retry |

## API Endpoints Overview

### Base URL
All endpoints use the base URL: `https://your-ssvirt-instance.com/cloudapi/1.0.0`

### Core Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `GET` | `/vdcs` | List accessible VDCs |
| `GET` | `/vdcs/{vdc_id}` | Get VDC details |
| `POST` | `/vdcs/{vdc_id}/actions/instantiateTemplate` | Create VM from template |
| `GET` | `/vdcs/{vdc_id}/vapps` | List vApps in VDC |
| `GET` | `/vapps/{vapp_id}` | Get vApp details |
| `DELETE` | `/vapps/{vapp_id}` | Delete vApp and VMs |
| `GET` | `/vms/{vm_id}` | Get VM details |
| `GET` | `/catalogs` | List accessible catalogs |
| `GET` | `/catalogs/{catalog_id}/catalogItems` | List templates in catalog |

## VM Creation Workflow

### Step 1: List Available VDCs

Get VDCs where the user can create VMs:

```http
GET /cloudapi/1.0.0/vdcs
```

**Response:**
```json
{
  "resultTotal": 3,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "values": [
    {
      "id": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
      "name": "Development VDC",
      "description": "Development environment",
      "allocationModel": "PayAsYouGo",
      "isEnabled": true
    }
  ]
}
```

### Step 2: List Available Templates

Get catalog items (templates) available for VM creation:

```http
GET /cloudapi/1.0.0/catalogs/{catalog_id}/catalogItems
```

**Response:**
```json
{
  "resultTotal": 5,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "values": [
    {
      "id": "ubuntu-22-04-server",
      "name": "Ubuntu 22.04 LTS Server",
      "description": "Ubuntu 22.04 LTS Server template",
      "catalogId": "urn:vcloud:catalog:11111111-2222-3333-4444-555555555555",
      "status": "RESOLVED",
      "entity": {
        "name": "Ubuntu 22.04 LTS Server",
        "description": "Ready-to-use Ubuntu server",
        "type": "vAppTemplate",
        "numberOfVMs": 1,
        "numberOfCpus": 2,
        "memoryAllocation": 4096,
        "storageAllocation": 20480
      }
    }
  ]
}
```

### Step 3: Create VM

Create a new VM by instantiating a template:

```http
POST /cloudapi/1.0.0/vdcs/{vdc_id}/actions/instantiateTemplate
Content-Type: application/json

{
  "name": "my-development-vm",
  "description": "Development VM for testing",
  "catalogItem": {
    "id": "ubuntu-22-04-server",
    "name": "Ubuntu 22.04 LTS Server"
  }
}
```

**Response:**
```json
{
  "id": "urn:vcloud:vapp:87654321-4321-4321-4321-210987654321",
  "name": "my-development-vm",
  "description": "Development VM for testing",
  "status": "INSTANTIATING",
  "vdcId": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
  "templateId": "urn:vcloud:template:11111111-2222-3333-4444-555555555555",
  "createdAt": "2024-01-15T10:30:00Z",
  "numberOfVMs": 1,
  "href": "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:87654321-4321-4321-4321-210987654321"
}
```

### Step 4: Monitor Creation Progress

Poll the vApp status to monitor VM creation:

```http
GET /cloudapi/1.0.0/vapps/{vapp_id}
```

**UI Implementation Pattern:**
- Poll every 5-10 seconds while status is `INSTANTIATING`
- Show progress indicator to user
- Display success when status becomes `RESOLVED` or `DEPLOYED`
- Handle errors if status becomes `FAILED`

## vApp Management

### List vApps in VDC

Display all vApps in a specific VDC:

```http
GET /cloudapi/1.0.0/vdcs/{vdc_id}/vapps?page=1&pageSize=25
```

**Query Parameters:**
- `page`: Page number (default: 1)
- `pageSize`: Items per page (default: 25, max: 100)
- `filter`: Filter expression (e.g., `name==my-vapp`)
- `sortAsc`: Sort field ascending
- `sortDesc`: Sort field descending

**Response:**
```json
{
  "resultTotal": 15,
  "pageCount": 1,
  "page": 1,
  "pageSize": 25,
  "values": [
    {
      "id": "urn:vcloud:vapp:87654321-4321-4321-4321-210987654321",
      "name": "my-development-vm",
      "description": "Development VM for testing",
      "status": "DEPLOYED",
      "vdcId": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
      "templateId": "urn:vcloud:template:11111111-2222-3333-4444-555555555555",
      "createdAt": "2024-01-15T10:30:00Z",
      "numberOfVMs": 1,
      "href": "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:87654321-4321-4321-4321-210987654321"
    }
  ]
}
```

### Get vApp Details

Get detailed information including contained VMs:

```http
GET /cloudapi/1.0.0/vapps/{vapp_id}
```

**Response:**
```json
{
  "id": "urn:vcloud:vapp:87654321-4321-4321-4321-210987654321",
  "name": "my-development-vm",
  "description": "Development VM for testing",
  "status": "DEPLOYED",
  "vdcId": "urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc",
  "templateId": "urn:vcloud:template:11111111-2222-3333-4444-555555555555",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:35:00Z",
  "numberOfVMs": 1,
  "vms": [
    {
      "id": "urn:vcloud:vm:11111111-2222-3333-4444-555555555555",
      "name": "my-development-vm-vm1",
      "status": "POWERED_ON",
      "href": "/cloudapi/1.0.0/vms/urn:vcloud:vm:11111111-2222-3333-4444-555555555555"
    }
  ],
  "href": "/cloudapi/1.0.0/vapps/urn:vcloud:vapp:87654321-4321-4321-4321-210987654321"
}
```

### Delete vApp

Remove vApp and all contained VMs:

```http
DELETE /cloudapi/1.0.0/vapps/{vapp_id}?force=true
```

**Query Parameters:**
- `force`: Force deletion even if VMs are powered on (default: false)

**Success Response:** `204 No Content`

## VM Details

### Get VM Information

Get comprehensive VM details including runtime status:

```http
GET /cloudapi/1.0.0/vms/{vm_id}
```

**Response:**
```json
{
  "id": "urn:vcloud:vm:11111111-2222-3333-4444-555555555555",
  "name": "my-development-vm-vm1",
  "description": "Virtual machine created from Ubuntu template",
  "status": "POWERED_ON",
  "vappId": "urn:vcloud:vapp:87654321-4321-4321-4321-210987654321",
  "templateId": "urn:vcloud:template:11111111-2222-3333-4444-555555555555",
  "createdAt": "2024-01-15T10:30:00Z",
  "updatedAt": "2024-01-15T10:35:00Z",
  "guestOs": "Ubuntu Linux (64-bit)",
  "vmTools": {
    "status": "RUNNING",
    "version": "12.1.5"
  },
  "hardware": {
    "numCpus": 2,
    "numCoresPerSocket": 1,
    "memoryMB": 4096
  },
  "storageProfile": {
    "name": "default-storage-policy",
    "href": "/cloudapi/1.0.0/storageProfiles/default-storage-policy"
  },
  "networkConnections": [
    {
      "networkName": "default-network",
      "ipAddress": "192.168.1.100",
      "macAddress": "00:50:56:12:34:56",
      "connected": true
    }
  ],
  "href": "/cloudapi/1.0.0/vms/urn:vcloud:vm:11111111-2222-3333-4444-555555555555"
}
```

## Error Handling

### HTTP Status Codes

| Code | Meaning | Common Causes |
|------|---------|---------------|
| `400` | Bad Request | Invalid request format, missing required fields |
| `401` | Unauthorized | Missing or invalid JWT token |
| `403` | Forbidden | User lacks permission for requested resource |
| `404` | Not Found | Resource doesn't exist or user can't access it |
| `409` | Conflict | Resource name conflict, operation not allowed |
| `500` | Internal Server Error | Server-side failure, OpenShift integration issues |

### Error Response Format

```json
{
  "code": 400,
  "error": "Bad Request",
  "message": "Invalid VDC URN format",
  "details": "VDC URN must follow format: urn:vcloud:vdc:{uuid}"
}
```

### Common Error Scenarios

#### VM Creation Errors

```json
{
  "code": 404,
  "error": "Not Found",
  "message": "Catalog item not found",
  "details": "Catalog item 'ubuntu-22-04-server' does not exist or is not accessible"
}
```

#### Access Control Errors

```json
{
  "code": 403,
  "error": "Forbidden",
  "message": "VDC access denied",
  "details": "User is not a member of the organization that owns this VDC"
}
```

#### Resource Conflict Errors

```json
{
  "code": 409,
  "error": "Conflict",
  "message": "Name already in use within VDC",
  "details": "A vApp with name 'my-development-vm' already exists in this VDC"
}
```

## UI Implementation Patterns

### VM Creation Form

```javascript
// VM Creation Form Component
class VMCreationForm {
  constructor() {
    this.vdcs = [];
    this.catalogItems = [];
    this.selectedVDC = null;
    this.selectedTemplate = null;
  }

  async loadVDCs() {
    try {
      const response = await fetch('/cloudapi/1.0.0/vdcs', {
        headers: { 'Authorization': `Bearer ${this.jwtToken}` }
      });
      const data = await response.json();
      this.vdcs = data.values;
    } catch (error) {
      this.handleError('Failed to load VDCs', error);
    }
  }

  async loadCatalogItems(catalogId) {
    try {
      const response = await fetch(`/cloudapi/1.0.0/catalogs/${catalogId}/catalogItems`, {
        headers: { 'Authorization': `Bearer ${this.jwtToken}` }
      });
      const data = await response.json();
      this.catalogItems = data.values;
    } catch (error) {
      this.handleError('Failed to load templates', error);
    }
  }

  async createVM(formData) {
    try {
      const response = await fetch(`/cloudapi/1.0.0/vdcs/${formData.vdcId}/actions/instantiateTemplate`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${this.jwtToken}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          name: formData.name,
          description: formData.description,
          catalogItem: {
            id: formData.templateId,
            name: formData.templateName
          }
        })
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      const vapp = await response.json();
      this.monitorVMCreation(vapp.id);
      return vapp;
    } catch (error) {
      this.handleError('Failed to create VM', error);
    }
  }

  async monitorVMCreation(vappId) {
    const pollInterval = 5000; // 5 seconds
    const maxAttempts = 60; // 5 minutes maximum
    let attempts = 0;

    const poll = async () => {
      try {
        const response = await fetch(`/cloudapi/1.0.0/vapps/${vappId}`, {
          headers: { 'Authorization': `Bearer ${this.jwtToken}` }
        });
        const vapp = await response.json();

        this.updateCreationProgress(vapp);

        if (vapp.status === 'DEPLOYED' || vapp.status === 'RESOLVED') {
          this.onVMCreationSuccess(vapp);
        } else if (vapp.status === 'FAILED') {
          this.onVMCreationFailure(vapp);
        } else if (attempts < maxAttempts) {
          attempts++;
          setTimeout(poll, pollInterval);
        } else {
          this.onVMCreationTimeout(vapp);
        }
      } catch (error) {
        this.handleError('Failed to check VM creation status', error);
      }
    };

    poll();
  }
}
```

### vApp List Component

```javascript
// vApp List Component
class VAppList {
  constructor() {
    this.vapps = [];
    this.currentPage = 1;
    this.pageSize = 25;
    this.totalPages = 1;
    this.loading = false;
  }

  async loadVApps(vdcId, page = 1) {
    this.loading = true;
    try {
      const response = await fetch(
        `/cloudapi/1.0.0/vdcs/${vdcId}/vapps?page=${page}&pageSize=${this.pageSize}`,
        {
          headers: { 'Authorization': `Bearer ${this.jwtToken}` }
        }
      );
      const data = await response.json();
      
      this.vapps = data.values;
      this.currentPage = data.page;
      this.totalPages = data.pageCount;
      this.updateUI();
    } catch (error) {
      this.handleError('Failed to load vApps', error);
    } finally {
      this.loading = false;
    }
  }

  async deleteVApp(vappId, forcePowerOff = false) {
    if (!confirm('Are you sure you want to delete this vApp and all its VMs?')) {
      return;
    }

    try {
      const response = await fetch(`/cloudapi/1.0.0/vapps/${vappId}?force=${forcePowerOff}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${this.jwtToken}` }
      });

      if (response.status === 204) {
        this.onVAppDeleted(vappId);
        this.loadVApps(this.currentVDC, this.currentPage);
      } else if (response.status === 400) {
        const error = await response.json();
        if (error.message.includes('running VMs')) {
          this.promptForceDelete(vappId);
        } else {
          throw new Error(error.message);
        }
      } else {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
    } catch (error) {
      this.handleError('Failed to delete vApp', error);
    }
  }

  promptForceDelete(vappId) {
    if (confirm('This vApp contains running VMs. Force power off and delete?')) {
      this.deleteVApp(vappId, true);
    }
  }
}
```

### VM Details Component

```javascript
// VM Details Component
class VMDetails {
  constructor() {
    this.vm = null;
    this.refreshInterval = null;
  }

  async loadVMDetails(vmId) {
    try {
      const response = await fetch(`/cloudapi/1.0.0/vms/${vmId}`, {
        headers: { 'Authorization': `Bearer ${this.jwtToken}` }
      });
      const vm = await response.json();
      
      this.vm = vm;
      this.updateUI();
      this.startAutoRefresh();
    } catch (error) {
      this.handleError('Failed to load VM details', error);
    }
  }

  startAutoRefresh() {
    // Refresh VM status every 30 seconds
    this.refreshInterval = setInterval(() => {
      this.loadVMDetails(this.vm.id);
    }, 30000);
  }

  stopAutoRefresh() {
    if (this.refreshInterval) {
      clearInterval(this.refreshInterval);
      this.refreshInterval = null;
    }
  }

  formatVMStatus(status) {
    const statusMap = {
      'POWERED_ON': { text: 'Powered On', class: 'status-running', icon: 'play' },
      'POWERED_OFF': { text: 'Powered Off', class: 'status-stopped', icon: 'stop' },
      'POWERING_ON': { text: 'Starting...', class: 'status-pending', icon: 'loading' },
      'POWERING_OFF': { text: 'Stopping...', class: 'status-pending', icon: 'loading' },
      'SUSPENDED': { text: 'Suspended', class: 'status-paused', icon: 'pause' },
      'UNKNOWN': { text: 'Unknown', class: 'status-error', icon: 'question' }
    };
    return statusMap[status] || statusMap['UNKNOWN'];
  }
}
```

## Code Examples

### Complete VM Creation Workflow

```javascript
class VMManagementClient {
  constructor(baseUrl, jwtToken) {
    this.baseUrl = baseUrl;
    this.jwtToken = jwtToken;
  }

  async createVMWorkflow(vdcId, catalogId, vmConfig) {
    try {
      // Step 1: Validate VDC access
      const vdc = await this.getVDC(vdcId);
      console.log(`Creating VM in VDC: ${vdc.name}`);

      // Step 2: Get available templates
      const templates = await this.getCatalogItems(catalogId);
      const template = templates.find(t => t.id === vmConfig.templateId);
      if (!template) {
        throw new Error(`Template ${vmConfig.templateId} not found`);
      }

      // Step 3: Create VM
      const vapp = await this.createVM(vdcId, {
        name: vmConfig.name,
        description: vmConfig.description,
        catalogItem: {
          id: template.id,
          name: template.name
        }
      });

      console.log(`VM creation started. vApp ID: ${vapp.id}`);

      // Step 4: Monitor creation
      const finalVApp = await this.waitForVMCreation(vapp.id);
      console.log(`VM creation completed. Status: ${finalVApp.status}`);

      return finalVApp;
    } catch (error) {
      console.error('VM creation failed:', error);
      throw error;
    }
  }

  async getVDC(vdcId) {
    const response = await this.request('GET', `/vdcs/${vdcId}`);
    return response;
  }

  async getCatalogItems(catalogId) {
    const response = await this.request('GET', `/catalogs/${catalogId}/catalogItems`);
    return response.values;
  }

  async createVM(vdcId, vmData) {
    const response = await this.request('POST', `/vdcs/${vdcId}/actions/instantiateTemplate`, vmData);
    return response;
  }

  async waitForVMCreation(vappId, maxWaitTime = 300000) { // 5 minutes
    const startTime = Date.now();
    const pollInterval = 5000; // 5 seconds

    while (Date.now() - startTime < maxWaitTime) {
      const vapp = await this.request('GET', `/vapps/${vappId}`);
      
      if (vapp.status === 'DEPLOYED' || vapp.status === 'RESOLVED') {
        return vapp;
      } else if (vapp.status === 'FAILED') {
        throw new Error(`VM creation failed. vApp status: ${vapp.status}`);
      }

      await this.sleep(pollInterval);
    }

    throw new Error('VM creation timed out');
  }

  async request(method, endpoint, data = null) {
    const url = `${this.baseUrl}/cloudapi/1.0.0${endpoint}`;
    const options = {
      method,
      headers: {
        'Authorization': `Bearer ${this.jwtToken}`,
        'Content-Type': 'application/json'
      }
    };

    if (data && (method === 'POST' || method === 'PUT')) {
      options.body = JSON.stringify(data);
    }

    const response = await fetch(url, options);
    
    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.message || `HTTP ${response.status}: ${response.statusText}`);
    }

    if (response.status === 204) {
      return null; // No content
    }

    return await response.json();
  }

  sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
}

// Usage Example
const client = new VMManagementClient('https://ssvirt.example.com', userJwtToken);

client.createVMWorkflow('urn:vcloud:vdc:12345678-1234-1234-1234-123456789abc', 'my-catalog-id', {
  name: 'test-vm',
  description: 'Test virtual machine',
  templateId: 'ubuntu-22-04-server'
}).then(vapp => {
  console.log('VM created successfully:', vapp);
}).catch(error => {
  console.error('VM creation failed:', error);
});
```

### React Component Example

```jsx
import React, { useState, useEffect } from 'react';

const VMCreationForm = ({ jwtToken, onVMCreated, onError }) => {
  const [vdcs, setVdcs] = useState([]);
  const [templates, setTemplates] = useState([]);
  const [selectedVDC, setSelectedVDC] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState('');
  const [vmName, setVmName] = useState('');
  const [vmDescription, setVmDescription] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [creationProgress, setCreationProgress] = useState(null);

  const client = new VMManagementClient('https://ssvirt.example.com', jwtToken);

  useEffect(() => {
    loadVDCs();
    loadTemplates();
  }, []);

  const loadVDCs = async () => {
    try {
      const response = await client.request('GET', '/vdcs');
      setVdcs(response.values);
    } catch (error) {
      onError('Failed to load VDCs', error);
    }
  };

  const loadTemplates = async () => {
    try {
      // Assuming a default catalog - in practice, you'd load this dynamically
      const templates = await client.getCatalogItems('default-catalog');
      setTemplates(templates);
    } catch (error) {
      onError('Failed to load templates', error);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setIsCreating(true);
    setCreationProgress('Initializing...');

    try {
      const template = templates.find(t => t.id === selectedTemplate);
      
      setCreationProgress('Creating VM...');
      const vapp = await client.createVM(selectedVDC, {
        name: vmName,
        description: vmDescription,
        catalogItem: {
          id: template.id,
          name: template.name
        }
      });

      setCreationProgress('Monitoring creation progress...');
      const finalVApp = await client.waitForVMCreation(vapp.id);
      
      onVMCreated(finalVApp);
      
      // Reset form
      setVmName('');
      setVmDescription('');
      setSelectedTemplate('');
    } catch (error) {
      onError('VM creation failed', error);
    } finally {
      setIsCreating(false);
      setCreationProgress(null);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="vm-creation-form">
      <div className="form-group">
        <label htmlFor="vdc-select">Virtual Data Center:</label>
        <select
          id="vdc-select"
          value={selectedVDC}
          onChange={(e) => setSelectedVDC(e.target.value)}
          required
          disabled={isCreating}
        >
          <option value="">Select VDC...</option>
          {vdcs.map(vdc => (
            <option key={vdc.id} value={vdc.id}>{vdc.name}</option>
          ))}
        </select>
      </div>

      <div className="form-group">
        <label htmlFor="template-select">Template:</label>
        <select
          id="template-select"
          value={selectedTemplate}
          onChange={(e) => setSelectedTemplate(e.target.value)}
          required
          disabled={isCreating}
        >
          <option value="">Select template...</option>
          {templates.map(template => (
            <option key={template.id} value={template.id}>
              {template.name} ({template.entity.numberOfCpus} CPU, {template.entity.memoryAllocation}MB RAM)
            </option>
          ))}
        </select>
      </div>

      <div className="form-group">
        <label htmlFor="vm-name">VM Name:</label>
        <input
          type="text"
          id="vm-name"
          value={vmName}
          onChange={(e) => setVmName(e.target.value)}
          required
          disabled={isCreating}
          placeholder="Enter VM name..."
        />
      </div>

      <div className="form-group">
        <label htmlFor="vm-description">Description:</label>
        <textarea
          id="vm-description"
          value={vmDescription}
          onChange={(e) => setVmDescription(e.target.value)}
          disabled={isCreating}
          placeholder="Optional description..."
          rows={3}
        />
      </div>

      {creationProgress && (
        <div className="creation-progress">
          <div className="progress-bar">
            <div className="progress-indicator"></div>
          </div>
          <p>{creationProgress}</p>
        </div>
      )}

      <button 
        type="submit" 
        disabled={isCreating || !selectedVDC || !selectedTemplate || !vmName}
        className="create-vm-button"
      >
        {isCreating ? 'Creating VM...' : 'Create VM'}
      </button>
    </form>
  );
};

export default VMCreationForm;
```

This comprehensive guide provides UI developers with all the information needed to implement VM creation and management functionality using the SSVirt CloudAPI endpoints. The examples show both vanilla JavaScript and React implementations with proper error handling and user experience considerations.