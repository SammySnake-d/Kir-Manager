export namespace kiroprocess {
	
	export class ProcessInfo {
	    pid: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcessInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.name = source["name"];
	    }
	}

}

export namespace main {
	
	export class BackupItem {
	    name: string;
	    backupTime: string;
	    hasToken: boolean;
	    hasMachineId: boolean;
	    machineId: string;
	    provider: string;
	    isCurrent: boolean;
	    isOriginalMachine: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BackupItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.backupTime = source["backupTime"];
	        this.hasToken = source["hasToken"];
	        this.hasMachineId = source["hasMachineId"];
	        this.machineId = source["machineId"];
	        this.provider = source["provider"];
	        this.isCurrent = source["isCurrent"];
	        this.isOriginalMachine = source["isOriginalMachine"];
	    }
	}
	export class Result {
	    success: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	    }
	}
	export class SoftResetStatus {
	    isPatched: boolean;
	    hasCustomId: boolean;
	    customMachineId: string;
	    extensionPath: string;
	    isSupported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SoftResetStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isPatched = source["isPatched"];
	        this.hasCustomId = source["hasCustomId"];
	        this.customMachineId = source["customMachineId"];
	        this.extensionPath = source["extensionPath"];
	        this.isSupported = source["isSupported"];
	    }
	}

}

