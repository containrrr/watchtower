
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

export const logIn = async (password: string, remember: boolean) => {
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

export const checkLogin = async () => {
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

export const list = async () => {
    const response = await fetch(apiBasePath + "list", {
        headers: headers()
    });
    const data = await response.json();
    return data;
};

export const check = async (containerId: string) => {
    const requestData = {
        ContainerId: containerId
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
    return data;
};

export const update = async (images?: string[]) => {
    let updateUrl = new URL(apiBasePath + "/update");

    if (images instanceof Array) {
        images.map(image => updateUrl.searchParams.append("image", image));
    }

    const response = await fetch(updateUrl.toString(), {
        headers: headers(),
    });

    return response.ok;
};