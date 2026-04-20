export namespace chat {
	
	export class Error {
	    status_code: number;
	    code: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Error(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status_code = source["status_code"];
	        this.code = source["code"];
	        this.message = source["message"];
	    }
	}

}

export namespace main {
	
	export class ChatResult {
	    response?: string;
	    error?: chat.Error;
	
	    static createFrom(source: any = {}) {
	        return new ChatResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.response = source["response"];
	        this.error = this.convertValues(source["error"], chat.Error);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HistorySearchItem {
	    message_id: number;
	    session_id: string;
	    session_title: string;
	    role: string;
	    content: string;
	    status: string;
	    error_code: string;
	    created_at: number;
	
	    static createFrom(source: any = {}) {
	        return new HistorySearchItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message_id = source["message_id"];
	        this.session_id = source["session_id"];
	        this.session_title = source["session_title"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.status = source["status"];
	        this.error_code = source["error_code"];
	        this.created_at = source["created_at"];
	    }
	}

}

