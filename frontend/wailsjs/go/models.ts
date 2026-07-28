export namespace domain {
	
	export class AudioTranscriptionRequest {
	    path: string;
	    language?: string;
	
	    static createFrom(source: any = {}) {
	        return new AudioTranscriptionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.language = source["language"];
	    }
	}
	export class AudioTranscriptionResult {
	    text: string;
	    language?: string;
	
	    static createFrom(source: any = {}) {
	        return new AudioTranscriptionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.language = source["language"];
	    }
	}
	export class CapturedAudioTranscriptionRequest {
	    wavBase64: string;
	    language?: string;
	
	    static createFrom(source: any = {}) {
	        return new CapturedAudioTranscriptionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.wavBase64 = source["wavBase64"];
	        this.language = source["language"];
	    }
	}
	export class DocumentRequest {
	    path: string;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new DocumentRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.language = source["language"];
	    }
	}
	export class DocumentSegment {
	    ordinal: number;
	    original: string;
	    translation: string;
	
	    static createFrom(source: any = {}) {
	        return new DocumentSegment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ordinal = source["ordinal"];
	        this.original = source["original"];
	        this.translation = source["translation"];
	    }
	}
	export class DocumentResult {
	    segments: DocumentSegment[];
	    detected: number;
	    extracted: number;
	    translated: number;
	
	    static createFrom(source: any = {}) {
	        return new DocumentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.segments = this.convertValues(source["segments"], DocumentSegment);
	        this.detected = source["detected"];
	        this.extracted = source["extracted"];
	        this.translated = source["translated"];
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
	
	export class FileSelection {
	    path: string;
	    name: string;
	    size: number;
	    mimeType: string;
	    previewUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileSelection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.mimeType = source["mimeType"];
	        this.previewUrl = source["previewUrl"];
	    }
	}
	export class HuggingFaceFile {
	    name: string;
	    size: number;
	    oid?: string;
	    mmproj: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HuggingFaceFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	        this.oid = source["oid"];
	        this.mmproj = source["mmproj"];
	    }
	}
	export class HuggingFaceInstallRequest {
	    repository: string;
	    modelFile: string;
	    mmprojFile?: string;
	
	    static createFrom(source: any = {}) {
	        return new HuggingFaceInstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repository = source["repository"];
	        this.modelFile = source["modelFile"];
	        this.mmprojFile = source["mmprojFile"];
	    }
	}
	export class HuggingFaceModel {
	    id: string;
	    author: string;
	    downloads: number;
	    likes: number;
	    lastModified: string;
	    tags: string[];
	    pipelineTag: string;
	
	    static createFrom(source: any = {}) {
	        return new HuggingFaceModel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.author = source["author"];
	        this.downloads = source["downloads"];
	        this.likes = source["likes"];
	        this.lastModified = source["lastModified"];
	        this.tags = source["tags"];
	        this.pipelineTag = source["pipelineTag"];
	    }
	}
	export class ImageRequest {
	    path: string;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new ImageRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.language = source["language"];
	    }
	}
	export class ImageResult {
	    original: string;
	    translation: string;
	
	    static createFrom(source: any = {}) {
	        return new ImageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.original = source["original"];
	        this.translation = source["translation"];
	    }
	}
	export class LlamaCppInstalledRuntime {
	    version: string;
	    cpuInstalled: boolean;
	    cudaInstalled: boolean;
	    vulkanInstalled: boolean;
	    hipInstalled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LlamaCppInstalledRuntime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.cpuInstalled = source["cpuInstalled"];
	        this.cudaInstalled = source["cudaInstalled"];
	        this.vulkanInstalled = source["vulkanInstalled"];
	        this.hipInstalled = source["hipInstalled"];
	    }
	}
	export class LlamaCppRuntimeArtifact {
	    url?: string;
	    size: number;
	    sha256?: string;
	
	    static createFrom(source: any = {}) {
	        return new LlamaCppRuntimeArtifact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	    }
	}
	export class LlamaCppRelease {
	    version: string;
	    publishedAt?: string;
	    cpu: LlamaCppRuntimeArtifact;
	    cuda: LlamaCppRuntimeArtifact;
	    vulkan: LlamaCppRuntimeArtifact;
	    hip: LlamaCppRuntimeArtifact;
	
	    static createFrom(source: any = {}) {
	        return new LlamaCppRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.publishedAt = source["publishedAt"];
	        this.cpu = this.convertValues(source["cpu"], LlamaCppRuntimeArtifact);
	        this.cuda = this.convertValues(source["cuda"], LlamaCppRuntimeArtifact);
	        this.vulkan = this.convertValues(source["vulkan"], LlamaCppRuntimeArtifact);
	        this.hip = this.convertValues(source["hip"], LlamaCppRuntimeArtifact);
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
	
	export class LlamaCppRuntimeInstallRequest {
	    version: string;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new LlamaCppRuntimeInstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.mode = source["mode"];
	    }
	}
	export class LlamaCppRuntimeStatus {
	    root: string;
	    selectedVersion?: string;
	    installed: LlamaCppInstalledRuntime[];
	
	    static createFrom(source: any = {}) {
	        return new LlamaCppRuntimeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.selectedVersion = source["selectedVersion"];
	        this.installed = this.convertValues(source["installed"], LlamaCppInstalledRuntime);
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
	export class ModelAssignment {
	    id: string;
	    path?: string;
	    projectionPath?: string;
	    supportsVision: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModelAssignment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.projectionPath = source["projectionPath"];
	        this.supportsVision = source["supportsVision"];
	    }
	}
	export class LlamaCppSettings {
	    runtimeVersion?: string;
	    runtimeMode: string;
	    contextSize: number;
	    modelDir?: string;
	    translation: ModelAssignment;
	    vision: ModelAssignment;
	
	    static createFrom(source: any = {}) {
	        return new LlamaCppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runtimeVersion = source["runtimeVersion"];
	        this.runtimeMode = source["runtimeMode"];
	        this.contextSize = source["contextSize"];
	        this.modelDir = source["modelDir"];
	        this.translation = this.convertValues(source["translation"], ModelAssignment);
	        this.vision = this.convertValues(source["vision"], ModelAssignment);
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
	export class LocalizationForm {
	    category: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalizationForm(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = source["category"];
	        this.text = source["text"];
	    }
	}
	export class LocalizationEntry {
	    id: string;
	    key: string;
	    source: LocalizationForm[];
	    translation: LocalizationForm[];
	    plural: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LocalizationEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.key = source["key"];
	        this.source = this.convertValues(source["source"], LocalizationForm);
	        this.translation = this.convertValues(source["translation"], LocalizationForm);
	        this.plural = source["plural"];
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
	export class LocalizationFile {
	    path: string;
	    name: string;
	    format: string;
	    fingerprint: string;
	    entries: LocalizationEntry[];
	
	    static createFrom(source: any = {}) {
	        return new LocalizationFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.format = source["format"];
	        this.fingerprint = source["fingerprint"];
	        this.entries = this.convertValues(source["entries"], LocalizationEntry);
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
	export class LocalizationFileRequest {
	    path: string;
	    format: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalizationFileRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.format = source["format"];
	    }
	}
	
	export class LocalizationSaveRequest {
	    path: string;
	    format: string;
	    fingerprint: string;
	    language: string;
	    entries: LocalizationEntry[];
	    untranslatedMode: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalizationSaveRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.format = source["format"];
	        this.fingerprint = source["fingerprint"];
	        this.language = source["language"];
	        this.entries = this.convertValues(source["entries"], LocalizationEntry);
	        this.untranslatedMode = source["untranslatedMode"];
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
	export class LocalizationSaveResult {
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalizationSaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	    }
	}
	export class LocalizationTranslationRequest {
	    operationId: string;
	    path: string;
	    format: string;
	    fingerprint: string;
	    language: string;
	    entryIds: string[];
	
	    static createFrom(source: any = {}) {
	        return new LocalizationTranslationRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operationId = source["operationId"];
	        this.path = source["path"];
	        this.format = source["format"];
	        this.fingerprint = source["fingerprint"];
	        this.language = source["language"];
	        this.entryIds = source["entryIds"];
	    }
	}
	export class LocalizationTranslationResult {
	    entries: LocalizationEntry[];
	    translated: number;
	    failed: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new LocalizationTranslationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entries = this.convertValues(source["entries"], LocalizationEntry);
	        this.translated = source["translated"];
	        this.failed = source["failed"];
	        this.total = source["total"];
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
	
	export class ModelInfo {
	    id: string;
	    name: string;
	    path?: string;
	    projectionPath?: string;
	    size: number;
	    modifiedAt?: string;
	    family?: string;
	    parameters?: string;
	    quantization?: string;
	    supportsVision: boolean;
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.projectionPath = source["projectionPath"];
	        this.size = source["size"];
	        this.modifiedAt = source["modifiedAt"];
	        this.family = source["family"];
	        this.parameters = source["parameters"];
	        this.quantization = source["quantization"];
	        this.supportsVision = source["supportsVision"];
	        this.running = source["running"];
	    }
	}
	export class MuPDFInstalledRuntime {
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new MuPDFInstalledRuntime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	    }
	}
	export class MuPDFRelease {
	    version: string;
	    publishedAt?: string;
	    artifact: LlamaCppRuntimeArtifact;
	
	    static createFrom(source: any = {}) {
	        return new MuPDFRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.publishedAt = source["publishedAt"];
	        this.artifact = this.convertValues(source["artifact"], LlamaCppRuntimeArtifact);
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
	export class MuPDFRuntimeInstallRequest {
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new MuPDFRuntimeInstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	    }
	}
	export class MuPDFRuntimeStatus {
	    root: string;
	    selectedVersion?: string;
	    installed: MuPDFInstalledRuntime[];
	
	    static createFrom(source: any = {}) {
	        return new MuPDFRuntimeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.selectedVersion = source["selectedVersion"];
	        this.installed = this.convertValues(source["installed"], MuPDFInstalledRuntime);
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
	export class MuPDFSettings {
	    version?: string;
	
	    static createFrom(source: any = {}) {
	        return new MuPDFSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	    }
	}
	export class OllamaSettings {
	    endpoint: string;
	    executable?: string;
	    translation: ModelAssignment;
	    vision: ModelAssignment;
	
	    static createFrom(source: any = {}) {
	        return new OllamaSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.executable = source["executable"];
	        this.translation = this.convertValues(source["translation"], ModelAssignment);
	        this.vision = this.convertValues(source["vision"], ModelAssignment);
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
	export class PromptSettings {
	    translation: string;
	    detection: string;
	    ocr: string;
	    wordVariants: string;
	
	    static createFrom(source: any = {}) {
	        return new PromptSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.translation = source["translation"];
	        this.detection = source["detection"];
	        this.ocr = source["ocr"];
	        this.wordVariants = source["wordVariants"];
	    }
	}
	export class ProviderStatus {
	    provider: string;
	    available: boolean;
	    running: boolean;
	    message: string;
	    models: ModelInfo[];
	
	    static createFrom(source: any = {}) {
	        return new ProviderStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.available = source["available"];
	        this.running = source["running"];
	        this.message = source["message"];
	        this.models = this.convertValues(source["models"], ModelInfo);
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
	export class WhisperModelAssignment {
	    id: string;
	    path?: string;
	
	    static createFrom(source: any = {}) {
	        return new WhisperModelAssignment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	    }
	}
	export class WhisperCppSettings {
	    runtimeVersion?: string;
	    runtimeMode: string;
	    modelDir?: string;
	    model: WhisperModelAssignment;
	    language: string;
	    microphoneId?: string;
	    microphoneName?: string;
	
	    static createFrom(source: any = {}) {
	        return new WhisperCppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runtimeVersion = source["runtimeVersion"];
	        this.runtimeMode = source["runtimeMode"];
	        this.modelDir = source["modelDir"];
	        this.model = this.convertValues(source["model"], WhisperModelAssignment);
	        this.language = source["language"];
	        this.microphoneId = source["microphoneId"];
	        this.microphoneName = source["microphoneName"];
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
	export class Settings {
	    version: number;
	    activeProvider: string;
	    ollama: OllamaSettings;
	    llamaCpp: LlamaCppSettings;
	    whisperCpp: WhisperCppSettings;
	    mupdf: MuPDFSettings;
	    prompts: PromptSettings;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.activeProvider = source["activeProvider"];
	        this.ollama = this.convertValues(source["ollama"], OllamaSettings);
	        this.llamaCpp = this.convertValues(source["llamaCpp"], LlamaCppSettings);
	        this.whisperCpp = this.convertValues(source["whisperCpp"], WhisperCppSettings);
	        this.mupdf = this.convertValues(source["mupdf"], MuPDFSettings);
	        this.prompts = this.convertValues(source["prompts"], PromptSettings);
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
	export class TranslateRequest {
	    text: string;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new TranslateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.language = source["language"];
	    }
	}
	export class TranslationVariant {
	    target: string;
	    replacement: string;
	
	    static createFrom(source: any = {}) {
	        return new TranslationVariant(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.replacement = source["replacement"];
	    }
	}
	export class TranslationVariantsRequest {
	    sourceText: string;
	    targetContext: string;
	    markedTargetContext: string;
	    selectedText: string;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new TranslationVariantsRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceText = source["sourceText"];
	        this.targetContext = source["targetContext"];
	        this.markedTargetContext = source["markedTargetContext"];
	        this.selectedText = source["selectedText"];
	        this.language = source["language"];
	    }
	}
	export class TranslationVariantsResult {
	    variants: TranslationVariant[];
	
	    static createFrom(source: any = {}) {
	        return new TranslationVariantsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variants = this.convertValues(source["variants"], TranslationVariant);
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
	export class UpdateAvailability {
	    available: boolean;
	    version: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateAvailability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.version = source["version"];
	        this.url = source["url"];
	    }
	}
	export class WhisperCppInstalledRuntime {
	    version: string;
	    cpuInstalled: boolean;
	    cudaInstalled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WhisperCppInstalledRuntime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.cpuInstalled = source["cpuInstalled"];
	        this.cudaInstalled = source["cudaInstalled"];
	    }
	}
	export class WhisperCppRelease {
	    version: string;
	    publishedAt?: string;
	    cpu: LlamaCppRuntimeArtifact;
	    cuda: LlamaCppRuntimeArtifact;
	
	    static createFrom(source: any = {}) {
	        return new WhisperCppRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.publishedAt = source["publishedAt"];
	        this.cpu = this.convertValues(source["cpu"], LlamaCppRuntimeArtifact);
	        this.cuda = this.convertValues(source["cuda"], LlamaCppRuntimeArtifact);
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
	export class WhisperCppRuntimeInstallRequest {
	    version: string;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new WhisperCppRuntimeInstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.mode = source["mode"];
	    }
	}
	export class WhisperCppRuntimeStatus {
	    root: string;
	    selectedVersion?: string;
	    installed: WhisperCppInstalledRuntime[];
	
	    static createFrom(source: any = {}) {
	        return new WhisperCppRuntimeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.selectedVersion = source["selectedVersion"];
	        this.installed = this.convertValues(source["installed"], WhisperCppInstalledRuntime);
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
	
	export class WhisperModel {
	    id: string;
	    name: string;
	    path?: string;
	    size: number;
	    sha256?: string;
	    installed: boolean;
	    multilingual: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WhisperModel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	        this.installed = source["installed"];
	        this.multilingual = source["multilingual"];
	    }
	}
	
	export class WhisperModelInstallRequest {
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new WhisperModelInstallRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	    }
	}
	export class WhisperStatus {
	    available: boolean;
	    running: boolean;
	    message: string;
	    runtime?: string;
	    model?: string;
	
	    static createFrom(source: any = {}) {
	        return new WhisperStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.running = source["running"];
	        this.message = source["message"];
	        this.runtime = source["runtime"];
	        this.model = source["model"];
	    }
	}

}

export namespace options {
	
	export class SecondInstanceData {
	    Args: string[];
	    WorkingDirectory: string;
	
	    static createFrom(source: any = {}) {
	        return new SecondInstanceData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Args = source["Args"];
	        this.WorkingDirectory = source["WorkingDirectory"];
	    }
	}

}

