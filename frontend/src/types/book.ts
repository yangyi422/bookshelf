export interface Author { id:string; name:string; sort_name:string }
export interface Tag { id:string; name:string }
export interface BookFile { id:string; book_id:string; format:string; mime_type:string; original_name:string; file_size:number; sha256:string; page_count?:number }
export interface Book { id:string; title:string; subtitle:string; description:string; language:string; publisher:string; isbn:string; cover_path:string; reading_status:string; rating:number; created_at:string; files:BookFile[]; authors:Author[]; tags:Tag[] }
export interface BookPage { items:Book[]; total:number; page:number; page_size:number }
