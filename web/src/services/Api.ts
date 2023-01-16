export interface ContainerListEntry {
    ContainerID: string;
    ContainerName: string;
    ImageName: string;
    ImageNameShort: string;
    ImageVersion: string;
    ImageCreatedDate: string;
}

export interface ListResponse {
    Containers: ContainerListEntry[];
}

export interface CheckRequest {
    ContainerID: string;
}

export interface CheckResponse {
    ContainerID: string;
    HasUpdate: boolean;
    NewVersion: string;
    NewVersionCreated: string;
}

const getEmbeddedVariable = (variableName: string) => {
    const value = document.body.getAttribute(`data-${variableName}`);
    if (value === null) {
        throw new Error(`No ${variableName} embedded variable detected`);
    }

    return value;
};

const apiBasePath = getEmbeddedVariable("apipath");
const tokenStorageKey = "token";
let token = "";

const headers = () => ({
    "Authorization": "Bearer " + token
});

export const logIn = async (password: string, remember: boolean): Promise<boolean> => {
    token = password;
    const response = await fetch(apiBasePath + "list", {
        headers: headers()
    });

    if (response.ok) {
        if (remember === true) {
            localStorage.setItem(tokenStorageKey, password);
        }
        return true;
    }

    token = "";
    return false;
};

export const checkLogin = async (): Promise<boolean> => {
    const savedToken = localStorage.getItem(tokenStorageKey);
    if (savedToken) {
        return await logIn(savedToken, false);
    }

    return false;
};

export const logOut = () => {
    token = "";
    localStorage.clear();
};

export const list = async (): Promise<ListResponse> => {
    const response = await fetch(apiBasePath + "list", {
        headers: headers()
    });
    const data = await response.json();
    return data as ListResponse;
};

export const check = async (containerId: string): Promise<CheckResponse> => {
    const requestData: CheckRequest = {
        ContainerID: containerId
    };
    const response = await fetch(apiBasePath + "check", {
        method: "POST",
        headers: {
            ...headers(),
            "Content-Type": "application/json",
        },
        body: JSON.stringify(requestData)
    });
    const data = await response.json();
    return data as CheckResponse;
};

export const update = async (containers?: string[]): Promise<boolean> => {
    let updateUrl = new URL(apiBasePath + "update");

    if (containers instanceof Array) {
        containers.map((container) => updateUrl.searchParams.append("container", container));
    }

    const response = await fetch(updateUrl.toString(), {
        headers: headers(),
    });

    return response.ok;
};