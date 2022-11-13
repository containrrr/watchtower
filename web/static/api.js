
function getEmbeddedVariable(variableName) {
    const value = document.body.getAttribute(`data-${variableName}`);
    if (value === null) {
        throw new Error(`No ${variableName} embedded variable detected`);
    }

    return value;
}

const apiFactory = () => {
    const apiBasePath = getEmbeddedVariable("apipath");
    let token = "";

    const headers = () => ({
        "Authorization": "Bearer " + token
    });

    const logIn = async (password, remember) => {
        token = password;
        const response = await fetch(apiBasePath + "list", {
            headers: headers()
        });

        if (response.ok) {
            if (remember === true) {
                localStorage.setItem("token", token);
            }

            return true;
        }

        token = "";
        return false;
    };

    const checkLogin = async () => {
        const token = localStorage.getItem("token");
        if (token) {
            return await logIn(token);
        }

        return false;
    };

    const logOut = () => {
        localStorage.clear();
    };

    const list = async () => {
        const response = await fetch(apiBasePath + "list", {
            headers: headers()
        });
        const data = await response.json();
        return data;
    };

    const check = async (containerId) => {
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

    const update = async (images) => {
        let updateUrl = new URL(apiBasePath + "/update");

        if (images instanceof Array) {
            images.map(image => updateUrl.searchParams.append("image", image));
        }

        const response = await fetch(updateUrl.toString(), {
            headers: headers(),
        });

        return response.ok;
    };

    return {
        checkLogin,
        logIn,
        logOut,
        list,
        check,
        update
    }
};